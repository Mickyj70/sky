package cli

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/Mickyj70/sky/internal/history"
	"github.com/Mickyj70/sky/internal/provider"
)

func TestNearbyNamedPlace(t *testing.T) {
	calls := 0
	source := sourceFunc(func(_ context.Context, q provider.Query) (provider.Result, error) {
		calls++
		if q.Latitude != 9.0068 || q.Longitude != 7.2632 || q.RadiusKM != 100 {
			t.Fatalf("unexpected lookup: %+v", q)
		}
		return provider.Result{}, nil
	})
	for _, args := range [][]string{{"nearby", "Abuja"}, {"nearby", "--state", "FCT"}, {"nearby", "Abuja", "--lat", "0"}} {
		cmd := NewWithSource(source)
		cmd.SetOut(io.Discard)
		cmd.SetArgs(args)
		err := Execute(context.Background(), cmd)
		if (err != nil) != (len(args) == 4) {
			t.Fatalf("%v: %v", args, err)
		}
	}
	if calls != 2 {
		t.Fatalf("expected two lookups, got %d", calls)
	}
}

func TestPlaceDayFiltersBeforePagination(t *testing.T) {
	dir, o := historyFixture(t)
	store, err := history.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	lat, lon := 9.0068, 7.2632
	// The existing fixture has no complete position. Local midnight in Abuja
	// is 23:00 UTC the previous day.
	for _, stamp := range []string{"2026-11-21T00:01:00Z", "2026-11-21T22:59:00Z", "2026-11-21T23:00:00Z"} {
		o.At, err = time.Parse(time.RFC3339, stamp)
		if err != nil {
			t.Fatal(err)
		}
		o.Result.SnapshotAt = &o.At
		o.Result.RetrievedAt = o.At
		o.Result.Aircraft[0].Latitude, o.Result.Aircraft[0].Longitude = &lat, &lon
		if _, err := store.Record(context.Background(), o, 720*time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	store.Close()
	for offset, want := range []string{"2026-11-21T00:01:00Z", "2026-11-21T22:59:00Z"} {
		args := []string{"history", "Abuja", "--day", "2026-11-21", "--data-dir", dir, "--limit", "1", "--json"}
		if offset == 1 {
			args = append(args, "--offset", "1")
		}
		out, err := runHistory(t, args...)
		var page history.Page
		if err != nil || json.Unmarshal([]byte(out), &page) != nil || len(page.Records) != 1 ||
			page.Records[0].Observation.At.Format(time.RFC3339) != want || page.HasMore != (offset == 0) {
			t.Fatalf("unexpected page %s, error %v", out, err)
		}
	}
}
