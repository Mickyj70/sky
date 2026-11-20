package history

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mickyj70/sky/internal/provider"
	"github.com/Mickyj70/sky/internal/storage"
	"github.com/Mickyj70/sky/internal/tracking"
)

func sample(at time.Time) tracking.Observation {
	stamp := at.Add(-time.Second)
	return tracking.Observation{At: at, Target: tracking.Target{Kind: provider.ByRegistration, Identifier: "G-TEST"}, State: tracking.Missing, Result: &provider.Result{Source: "fixture", RetrievedAt: at, SnapshotAt: &stamp, Aircraft: []provider.Aircraft{}}}
}
func openTest(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func count(t *testing.T, s *Store, want int64) {
	t.Helper()
	stats, err := s.Stats(context.Background())
	if err != nil || stats.Count != want || stats.Bytes <= 0 {
		t.Fatalf("stats=%+v err=%v want=%d", stats, err, want)
	}
}
func record(t *testing.T, s *Store, o tracking.Observation, want bool) {
	t.Helper()
	changed, err := s.Record(context.Background(), o, 24*time.Hour)
	if err != nil || changed != want {
		t.Fatalf("record changed=%t err=%v", changed, err)
	}
}
func TestPersistenceReplayAndRetention(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := openTest(t, dir)
	at := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	o := sample(at)
	record(t, s, o, true)
	s.Close()
	s = openTest(t, dir)
	o.At = o.At.Add(time.Hour)
	o.Result.RetrievedAt = o.At
	o.State = tracking.Stale
	record(t, s, o, false) // Same provider snapshot, later local time/state.
	var payload string
	if err := s.db.QueryRowContext(ctx, "SELECT payload FROM observations").Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if !containsTimestamp(payload, at) {
		t.Fatal("first encounter lost")
	}
	o = sample(at.Add(time.Hour))
	record(t, s, o, true) // New snapshot, same position.
	o.Target = tracking.Target{Kind: provider.ByHex, Identifier: "ABC123"}
	record(t, s, o, true)
	count(t, s, 3)
	o = sample(at.Add(25 * time.Hour))
	record(t, s, o, true)
	count(t, s, 3) // Strict cutoff removes first row; rows at cutoff remain.
	if err := s.db.QueryRowContext(ctx, "SELECT identifier FROM observations WHERE kind='hex'").Scan(&payload); err != nil || payload != "abc123" {
		t.Fatalf("canonical identifier: %q %v", payload, err)
	}
}
func containsTimestamp(payload string, at time.Time) bool {
	var o tracking.Observation
	return json.Unmarshal([]byte(payload), &o) == nil && o.At.Equal(at)
}

func TestFailedInsertRollsBackPrune(t *testing.T) {
	s := openTest(t, t.TempDir())
	at := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	record(t, s, sample(at), true)
	_, err := s.db.Exec(`CREATE TRIGGER fail_insert BEFORE INSERT ON observations BEGIN SELECT RAISE(ABORT, 'test disk failure'); END;`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Record(context.Background(), sample(at.Add(48*time.Hour)), time.Hour); err == nil {
		t.Fatal("expected failed insert")
	}
	count(t, s, 1)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.Record(canceled, sample(at.Add(72*time.Hour)), time.Hour); err == nil {
		t.Fatal("expected cancellation")
	}
	count(t, s, 1)
}

func TestRejectUnsupportedAndUnrelatedWithoutMutation(t *testing.T) {
	for _, ddl := range []string{"PRAGMA user_version=99", "PRAGMA application_id=3", "PRAGMA application_id=0; PRAGMA user_version=0;"} {
		t.Run(ddl, func(t *testing.T) {
			dir := t.TempDir()
			s := openTest(t, dir)
			record(t, s, sample(time.Now()), true)
			if _, err := s.db.Exec(ddl); err != nil {
				t.Fatal(err)
			}
			s.Close()
			path := filepath.Join(dir, Filename)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if other, err := Open(context.Background(), dir); err == nil {
				other.Close()
				t.Fatal("accepted incompatible db")
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) {
				t.Fatal("incompatible db changed")
			}
		})
	}
}

func TestHistoryBackupRecovery(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := openTest(t, dir)
	at := time.Now().UTC()
	record(t, s, sample(at), true)
	backup := filepath.Join(t.TempDir(), "history.db")
	if _, err := s.Backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Backup(ctx, backup); err == nil {
		t.Fatal("overwrote snapshot")
	}
	record(t, s, sample(at.Add(time.Hour)), true)
	restored := t.TempDir()
	// Restoring history alongside an existing saved watchlist must preserve it.
	watch, err := storage.Open(ctx, restored)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = watch.Add(ctx, storage.Registration, "G-OTHER"); err != nil {
		t.Fatal(err)
	}
	watch.Close()
	path, n, err := Restore(ctx, backup, restored)
	if err != nil || n != 1 {
		t.Fatalf("restore: %s %d %v", path, n, err)
	}
	fresh := openTest(t, restored)
	count(t, fresh, 1)
	record(t, fresh, sample(at), false)
	if _, _, err = Restore(ctx, backup, restored); err == nil {
		t.Fatal("overwrote active history")
	}
	count(t, fresh, 1)
	watch, err = storage.Open(ctx, restored)
	if err != nil {
		t.Fatal(err)
	}
	defer watch.Close()
	targets, err := watch.List(ctx)
	if err != nil || len(targets) != 1 || targets[0].Identifier != "G-OTHER" {
		t.Fatal("watchlist changed")
	}
	// Wrong identities/payloads and unsupported schemas cannot be published.
	damaged, err := connect(backup, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = damaged.Exec("UPDATE observations SET identifier='G-TAMPER'"); err != nil {
		t.Fatal(err)
	}
	damaged.Close()
	badDir := t.TempDir()
	if _, _, err = Restore(ctx, backup, badDir); err == nil {
		t.Fatal("accepted corrupt row")
	}
	if _, err = os.Stat(filepath.Join(badDir, Filename)); !os.IsNotExist(err) {
		t.Fatal("published invalid history")
	}
	files, err := os.ReadDir(badDir)
	if err != nil || len(files) != 0 {
		t.Fatalf("staging leaked: %v %v", files, err)
	}
}

func TestInvalidInputDoesNotPrune(t *testing.T) {
	s := openTest(t, t.TempDir())
	at := time.Now().UTC()
	record(t, s, sample(at), true)
	invalid := sample(at.Add(48 * time.Hour))
	invalid.Failure = &tracking.Failure{Message: "upstream unavailable"}
	if _, err := s.Record(context.Background(), invalid, time.Hour); err == nil {
		t.Fatal("stored failure")
	}
	invalid = sample(at)
	invalid.Target.Identifier = "bad/id"
	if _, err := s.Record(context.Background(), invalid, time.Hour); err == nil {
		t.Fatal("stored invalid target")
	}
	if _, err := s.Record(context.Background(), sample(at), 0); err == nil {
		t.Fatal("accepted zero retention")
	}
	count(t, s, 1)
}

func TestOpenDoesNotNeedWriterLock(t *testing.T) {
	dir := t.TempDir()
	s := openTest(t, dir)
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM observations"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	reader, err := Open(ctx, dir)
	if err != nil {
		t.Fatalf("opening valid history needed writer lock: %v", err)
	}
	defer reader.Close()
}

func TestRecoveryDoesNotImportExtraSchema(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	at := time.Now().UTC()
	record(t, s, sample(at), true)
	// A trigger on a snapshot must never become executable restored schema.
	if _, err := s.db.Exec(`CREATE TRIGGER surprise BEFORE INSERT ON observations BEGIN SELECT RAISE(ABORT,'must not be imported'); END;`); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "snapshot.db")
	if _, err := s.Backup(ctx, path); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, _, err := Restore(ctx, path, dir); err != nil {
		t.Fatal(err)
	}
	recovered := openTest(t, dir)
	record(t, recovered, sample(at.Add(time.Hour)), true)
	count(t, recovered, 2)
}
