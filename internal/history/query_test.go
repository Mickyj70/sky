package history

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Mickyj70/sky/internal/provider"
	"github.com/Mickyj70/sky/internal/storage"
)

func TestQueryBoundsAliasesPaginationAndParity(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	at := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	a := sample(at)
	a.Result.Source = "adsb.lol"
	record(t, s, a, true)
	b := sample(at)
	b.Target.Identifier = "G-OTHER"
	record(t, s, b, true) // tied encounter
	c := sample(at.Add(time.Hour))
	c.Target.Kind = provider.ByHex
	c.Target.Identifier = "ABC123"
	record(t, s, c, true)
	d := sample(at.Add(2 * time.Hour))
	record(t, s, d, true)
	all, err := s.Query(ctx, Filter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Records) != 4 || all.HasMore || len(all.Attribution) != 2 {
		t.Fatalf("page=%+v", all)
	}
	for i := 1; i < len(all.Records); i++ {
		left, right := all.Records[i-1], all.Records[i]
		if left.Observation.At.After(right.Observation.At) || (left.Observation.At.Equal(right.Observation.At) && left.Key >= right.Key) {
			t.Fatal("not deterministic")
		}
	}
	var gathered []Record
	for offset := 0; ; {
		page, err := s.Query(ctx, Filter{Limit: 1, Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		gathered = append(gathered, page.Records...)
		if !page.HasMore {
			if page.NextOffset != nil {
				t.Fatal("unnecessary continuation")
			}
			break
		}
		if page.NextOffset == nil {
			t.Fatal("missing continuation")
		}
		offset = *page.NextOffset
	}
	if !reflect.DeepEqual(gathered, all.Records) {
		t.Fatal("pagination changed records")
	}
	until := at.Add(2 * time.Hour)
	page, err := s.Query(ctx, Filter{Limit: 10, Since: &at, Until: &until})
	if err != nil || len(page.Records) != 3 {
		t.Fatalf("bounds: %+v %v", page, err)
	}
	page, err = s.Query(ctx, Filter{Limit: 10, Kind: storage.Hex, Identifier: "ABC123"})
	if err != nil || len(page.Records) != 1 || page.Records[0].Observation.Target.Identifier != "abc123" {
		t.Fatalf("hex: %+v %v", page, err)
	}
	page, err = s.Query(ctx, Filter{Limit: 10, Kind: storage.Registration, Identifier: "g-test"})
	if err != nil || len(page.Records) != 2 {
		t.Fatalf("registration: %+v %v", page, err)
	}
	page, err = s.Query(ctx, Filter{Limit: 10, Offset: 100})
	if err != nil || page.Records == nil || len(page.Records) != 0 || page.Attribution == nil {
		t.Fatal("empty page contract")
	}
	// Compare the stored payload, not just row counts.
	for _, r := range all.Records {
		var saved string
		if err = s.db.QueryRow("SELECT payload FROM observations WHERE key=?", r.Key).Scan(&saved); err != nil {
			t.Fatal(err)
		}
		want, err := json.Marshal(r.Observation)
		if err != nil || saved != string(want) {
			t.Fatal("query changed stored payload")
		}
	}
}
func TestQueryRejectsBadBoundsAndCorruption(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	at := time.Now().UTC()
	record(t, s, sample(at), true)
	invalid := []Filter{{Limit: 0}, {Limit: 1001}, {Limit: 1, Offset: -1}, {Limit: 1, Offset: MaxOffset + 1}, {Limit: 1, Kind: storage.Hex, Identifier: "bad"}, {Limit: 1, Since: &at, Until: &at}}
	for _, f := range invalid {
		if _, err := s.Query(ctx, f); err == nil {
			t.Fatalf("accepted %+v", f)
		}
	}
	if _, err := s.db.Exec("UPDATE observations SET payload='{' "); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Query(ctx, Filter{Limit: 10}); err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("corruption: %v", err)
	}
	if _, err := s.db.Exec("PRAGMA user_version=99"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Query(ctx, Filter{Limit: 10}); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("schema: %v", err)
	}
}
func TestPrunePreviewApplyBoundariesAndRollback(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	at := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		record(t, s, sample(at.Add(time.Duration(i)*time.Hour)), true)
	}
	cutoff := at.Add(time.Hour)
	preview, err := s.Prune(ctx, cutoff, false)
	if err != nil || preview.Matched != 1 || preview.Deleted != 0 || preview.Applied {
		t.Fatalf("preview=%+v %v", preview, err)
	}
	count(t, s, 3)
	if _, err = s.db.Exec(`CREATE TRIGGER fail_prune BEFORE DELETE ON observations BEGIN SELECT RAISE(ABORT,'test failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Prune(ctx, cutoff, true); err == nil {
		t.Fatal("expected prune failure")
	}
	count(t, s, 3)
	if _, err = s.db.Exec("DROP TRIGGER fail_prune"); err != nil {
		t.Fatal(err)
	}
	applied, err := s.Prune(ctx, cutoff, true)
	if err != nil || applied.Matched != 1 || applied.Deleted != 1 || !applied.Applied {
		t.Fatalf("apply=%+v %v", applied, err)
	}
	count(t, s, 2)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = s.Prune(canceled, at.Add(10*time.Hour), true); err == nil {
		t.Fatal("ignored cancellation")
	}
	count(t, s, 2)
}
func TestQueryIndexes(t *testing.T) {
	s := openTest(t, t.TempDir())
	for _, tc := range []struct {
		sql, index string
		args       []any
	}{
		{"SELECT key FROM observations WHERE at_ns>=? AND at_ns<? ORDER BY at_ns,key LIMIT ?", "observations_time", []any{0, 100, 10}},
		{"SELECT key FROM observations WHERE kind=? AND identifier=? AND at_ns>=? ORDER BY at_ns,key LIMIT ?", "observations_target_time", []any{"hex", "abc123", 0, 10}},
		{"SELECT count(*) FROM observations WHERE at_ns<?", "observations_time", []any{100}},
	} {
		rows, err := s.db.Query("EXPLAIN QUERY PLAN "+tc.sql, tc.args...)
		if err != nil {
			t.Fatal(err)
		}
		details := ""
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err = rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			details += detail
		}
		err = rows.Err()
		rows.Close()
		if err != nil || !strings.Contains(details, tc.index) || strings.Contains(details, "TEMP B-TREE") {
			t.Fatalf("query plan %q err=%v", details, err)
		}
	}
}

func TestQueryPageByteLimit(t *testing.T) {
	s := openTest(t, t.TempDir())
	at := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	// Each valid row fits, but the combined page exceeds the payload cap.
	for i := 0; i < 2; i++ {
		o := sample(at.Add(time.Duration(i) * time.Hour))
		o.Result.Source = strings.Repeat("x", MaxPageBytes/2)
		record(t, s, o, true)
	}
	if _, err := s.Query(context.Background(), Filter{Limit: 2}); err == nil || !strings.Contains(err.Error(), "lower --limit") {
		t.Fatalf("page cap: %v", err)
	}
	if page, err := s.Query(context.Background(), Filter{Limit: 1}); err != nil || len(page.Records) != 1 || !page.HasMore {
		t.Fatalf("smaller page failed: %v", err)
	}
}
