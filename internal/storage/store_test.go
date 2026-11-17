package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"testing"
)

func openTestStore(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPersistenceAndCanonicalDuplicates(t *testing.T) {
	ctx := context.Background()
	name := "data ?#"
	if runtime.GOOS == "windows" {
		name = "data # spaces"
	}
	dir := filepath.Join(t.TempDir(), name)
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	first, changed, err := s.Add(ctx, Registration, " g-zbke ")
	if err != nil || !changed {
		t.Fatalf("add: %v %v", changed, err)
	}
	duplicate, changed, err := s.Add(ctx, Registration, "G-ZBKE")
	if err != nil || changed || !first.CreatedAt.Equal(duplicate.CreatedAt) {
		t.Fatalf("duplicate: %+v %v %v", duplicate, changed, err)
	}
	if _, _, err := s.Add(ctx, Hex, "406D7B"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openTestStore(t, dir)
	saved, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 2 || saved[0].Kind != Hex || saved[0].Identifier != "406d7b" || saved[1].Identifier != "G-ZBKE" {
		t.Fatalf("unexpected saved targets: %+v", saved)
	}
	for i := 0; i < 2; i++ {
		removed, err := s.Remove(ctx, Hex, "406D7B")
		if err != nil || removed != (i == 0) {
			t.Fatalf("remove: %v %v", removed, err)
		}
	}
	saved, err = s.List(ctx)
	if err != nil || len(saved) != 1 {
		t.Fatalf("remaining targets: %+v %v", saved, err)
	}
}

func TestEmptyAndInvalidInputs(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, t.TempDir())
	saved, err := s.List(ctx)
	if err != nil || saved == nil || len(saved) != 0 {
		t.Fatalf("empty list must be nonnil: %+v %v", saved, err)
	}
	for _, tc := range []struct {
		kind Kind
		id   string
	}{{Registration, ""}, {Registration, "../oops"}, {Hex, "123"}, {Hex, "~406d7b"}, {Kind("callsign"), "BAW7"}} {
		if _, _, err := s.Add(ctx, tc.kind, tc.id); err == nil {
			t.Errorf("accepted invalid add: %+v", tc)
		}
		if _, err := s.Remove(ctx, tc.kind, tc.id); err == nil {
			t.Errorf("accepted invalid remove: %+v", tc)
		}
	}
	saved, err = s.List(ctx)
	if err != nil || len(saved) != 0 {
		t.Fatal("invalid changes modified saved state")
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(ctx, file); err == nil {
		s.Close()
		t.Fatal("accepted a file as a directory")
	}
	content, err := os.ReadFile(file)
	if err != nil || string(content) != "preserve" {
		t.Fatal("path error modified existing file")
	}
}

func TestTransactionFailurePreservesState(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, t.TempDir())
	if _, _, err := s.Add(ctx, Registration, "G-ZBKE"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TRIGGER fail_insert BEFORE INSERT ON watch_targets BEGIN SELECT RAISE(ABORT, 'simulated failed write'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Add(ctx, Hex, "406d7b"); err == nil {
		t.Fatal("expected insertion failure")
	}
	saved, err := s.List(ctx)
	if err != nil || len(saved) != 1 || saved[0].Identifier != "G-ZBKE" {
		t.Fatalf("failed insertion changed state: %+v %v", saved, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.Remove(canceled, Registration, "G-ZBKE"); err == nil {
		t.Fatal("canceled delete succeeded")
	}
	saved, err = s.List(ctx)
	if err != nil || len(saved) != 1 {
		t.Fatal("canceled deletion changed state")
	}
}

func TestConcurrentDuplicateWriters(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	a := openTestStore(t, dir)
	b := openTestStore(t, dir)
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	changes := make(chan bool, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := a
			if i%2 == 0 {
				s = b
			}
			_, changed, err := s.Add(ctx, Registration, "G-ZBKE")
			errs <- err
			changes <- changed
		}(i)
	}
	wg.Wait()
	close(errs)
	close(changes)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for changed := range changes {
		if changed {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected one inserted target, got %d", count)
	}
}

func TestRefuseUnknownSchemasWithoutChangingData(t *testing.T) {
	ctx := context.Background()
	for _, sql := range []string{`PRAGMA user_version=2`, `PRAGMA application_id=12345`} {
		t.Run(sql, func(t *testing.T) {
			dir := t.TempDir()
			s, err := Open(ctx, dir)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := s.Add(ctx, Registration, "G-ZBKE"); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.ExecContext(ctx, sql); err != nil {
				t.Fatal(err)
			}
			app, version, err := identity(ctx, s.db)
			if err != nil {
				t.Fatal(err)
			}
			s.Close()
			if other, err := Open(ctx, dir); err == nil {
				other.Close()
				t.Fatal("accepted unsupported database")
			}
			db, err := connect(filepath.Join(dir, Filename), true)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			actualApp, actualVersion, err := identity(ctx, db)
			if err != nil || app != actualApp || version != actualVersion {
				t.Fatal("refusal changed schema identity")
			}
			saved, err := targets(ctx, db)
			if err != nil || len(saved) != 1 {
				t.Fatal("refusal changed saved data")
			}
		})
	}
	dir := t.TempDir()
	path := filepath.Join(dir, Filename)
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := connect(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE sqliteXcustom(value TEXT)"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if other, err := Open(ctx, dir); err == nil {
		other.Close()
		t.Fatal("accepted unrelated unmarked database")
	}
}

func TestBackupAndRestoreCommittedWAL(t *testing.T) {
	ctx := context.Background()
	source := openTestStore(t, t.TempDir())
	if _, err := source.db.ExecContext(ctx, "PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := source.Add(ctx, Registration, "G-ZBKE"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := source.Add(ctx, Hex, "406d7b"); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(source.Path() + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("expected uncheckpointed WAL: %v", err)
	}
	backup, err := source.Backup(ctx, filepath.Join(t.TempDir(), "backups", "watch.db"))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := source.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := source.Add(ctx, Registration, "N123AB"); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "restored")
	path, count, err := Restore(ctx, backup, dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || path != filepath.Join(dir, Filename) {
		t.Fatalf("unexpected restore result: %s %d", path, count)
	}
	restored := openTestStore(t, dir)
	actual, err := restored.List(ctx)
	if err != nil || !reflect.DeepEqual(expected, actual) {
		t.Fatalf("restore mismatch: %+v %+v %v", expected, actual, err)
	}
	sourceTargets, err := source.List(ctx)
	if err != nil || len(sourceTargets) != 3 {
		t.Fatal("restore changed source data")
	}
	stages, err := filepath.Glob(filepath.Join(dir, ".sky-restore-*"))
	if err != nil || len(stages) != 0 {
		t.Fatal("restore left temporary files")
	}
}

func TestBackupAndRestoreNeverOverwrite(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, t.TempDir())
	if _, _, err := s.Add(ctx, Registration, "G-ZBKE"); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "existing.db")
	if err := os.WriteFile(destination, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Backup(ctx, destination); err == nil {
		t.Fatal("overwrote backup destination")
	}
	if data, err := os.ReadFile(destination); err != nil || string(data) != "preserve" {
		t.Fatal("backup altered destination")
	}
	backup, err := s.Backup(ctx, filepath.Join(t.TempDir(), "valid.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Restore(ctx, backup, filepath.Dir(s.Path())); err == nil {
		t.Fatal("restored over active database")
	}
	dir := t.TempDir()
	sidecar := filepath.Join(dir, Filename+"-wal")
	if err := os.WriteFile(sidecar, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Restore(ctx, backup, dir); err == nil {
		t.Fatal("restored over stale sidecar")
	}
	if data, _ := os.ReadFile(sidecar); string(data) != "preserve" {
		t.Fatal("restore changed sidecar")
	}
	targets, err := s.List(ctx)
	if err != nil || len(targets) != 1 {
		t.Fatal("failed restore changed current watchlist")
	}
}

func TestInvalidAndCanceledRecovery(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, t.TempDir())
	invalid := filepath.Join(t.TempDir(), "broken.db")
	if err := os.WriteFile(invalid, []byte("not SQLite"), 0600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "new")
	if _, _, err := Restore(ctx, invalid, dir); err == nil {
		t.Fatal("accepted invalid backup")
	}
	if _, err := os.Stat(filepath.Join(dir, Filename)); !os.IsNotExist(err) {
		t.Fatal("invalid restore published a database")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	dest := filepath.Join(t.TempDir(), "canceled.db")
	if _, err := s.Backup(canceled, dest); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled backup, got %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("failed backup left a partial destination")
	}
	backup, err := s.Backup(ctx, filepath.Join(t.TempDir(), "valid.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Restore(canceled, backup, dir); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled restore, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, Filename)); !os.IsNotExist(err) {
		t.Fatal("canceled restore published a database")
	}
}

func TestConcurrentRestorePublishesOneCompleteDatabase(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, t.TempDir())
	if _, _, err := s.Add(ctx, Registration, "G-ZBKE"); err != nil {
		t.Fatal(err)
	}
	backup, err := s.Backup(ctx, filepath.Join(t.TempDir(), "snapshot.db"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _, err := Restore(ctx, backup, dir); results <- err }()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected one successful restore, got %d", successes)
	}
	recovered := openTestStore(t, dir)
	saved, err := recovered.List(ctx)
	if err != nil || len(saved) != 1 {
		t.Fatalf("published incomplete database: %+v %v", saved, err)
	}
}
