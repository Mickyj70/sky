package tracking

import (
	"errors"
	"testing"
	"time"

	"github.com/Mickyj70/sky/internal/provider"
)

func number(v float64) *float64 { return &v }
func observationAttempt(at time.Time) Attempt {
	snapshot := at
	return Attempt{At: at, Query: provider.Query{Kind: provider.ByHex, Identifier: "abc123"}, Result: provider.Result{Source: "test", RetrievedAt: at, SnapshotAt: &snapshot, Aircraft: []provider.Aircraft{{Hex: "abc123", Latitude: number(0), Longitude: number(0), PositionSeen: number(0), Seen: number(0)}}}}
}

func TestObservationTransitionsAndRepeatSuppression(t *testing.T) {
	start := time.Unix(1000, 0).UTC()
	observer, err := NewObserver(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	attempt := observationAttempt(start)
	event, changed, err := observer.Observe(attempt)
	if err != nil || !changed || event.State != Observed || event.PositionAgeSeconds == nil || *event.PositionAgeSeconds != 0 {
		t.Fatalf("first observation: %+v %v %v", event, changed, err)
	}
	// Only sample metadata changes; stable coordinates/values should not reprint.
	attempt.At = start.Add(10 * time.Second)
	attempt.Result.RetrievedAt = attempt.At
	attempt.Result.Aircraft[0].Seen = number(10)
	attempt.Result.Aircraft[0].PositionSeen = number(10)
	event, changed, err = observer.Observe(attempt)
	if err != nil || changed || event.State != Observed {
		t.Fatalf("repeat: %+v %v %v", event, changed, err)
	}
	// Snapshot age and provider position age both contribute to staleness.
	attempt.At = start.Add(51 * time.Second)
	event, changed, err = observer.Observe(attempt)
	if err != nil || !changed || event.State != Stale || *event.PositionAgeSeconds != 61 {
		t.Fatalf("stale transition: %+v %v %v", event, changed, err)
	}
	attempt.Err = &provider.Error{Kind: provider.Unavailable, Message: "provider unavailable"}
	event, changed, err = observer.Observe(attempt)
	if err != nil || !changed || event.State != Unknown || event.Failure == nil || event.Result != nil {
		t.Fatalf("outage: %+v %v %v", event, changed, err)
	}
	if _, changed, err := observer.Observe(attempt); err != nil || changed {
		t.Fatal("identical outage repeated")
	}
	attempt.Err = nil
	attempt.Result.Aircraft = []provider.Aircraft{}
	event, changed, err = observer.Observe(attempt)
	if err != nil || !changed || event.State != Missing || event.Failure != nil {
		t.Fatalf("empty response: %+v %v %v", event, changed, err)
	}
	attempt = observationAttempt(start.Add(70 * time.Second))
	event, changed, err = observer.Observe(attempt)
	if err != nil || !changed || event.State != Observed {
		t.Fatalf("recovery: %+v %v %v", event, changed, err)
	}
}

func TestFreshnessUnknownAndBoundaries(t *testing.T) {
	at := time.Unix(1000, 0).UTC()
	cases := []struct {
		name   string
		modify func(*Attempt)
		want   State
	}{
		{"no snapshot", func(a *Attempt) { a.Result.SnapshotAt = nil }, Unknown},
		{"no age", func(a *Attempt) { a.Result.Aircraft[0].PositionSeen = nil }, Unknown},
		{"no coordinates", func(a *Attempt) { a.Result.Aircraft[0].Latitude = nil }, Unknown},
		{"threshold is inclusive", func(a *Attempt) { a.Result.Aircraft[0].PositionSeen = number(60) }, Observed},
		{"above threshold", func(a *Attempt) { a.Result.Aircraft[0].PositionSeen = number(60.1) }, Stale},
		{"future snapshot", func(a *Attempt) { future := at.Add(time.Second); a.Result.SnapshotAt = &future }, Observed},
		{"empty", func(a *Attempt) { a.Result.Aircraft = nil }, Missing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observer, _ := NewObserver(time.Minute)
			attempt := observationAttempt(at)
			tc.modify(&attempt)
			event, changed, err := observer.Observe(attempt)
			if err != nil || !changed || event.State != tc.want {
				t.Fatalf("got %+v %v %v", event, changed, err)
			}
		})
	}
	if _, err := NewObserver(0); err == nil {
		t.Fatal("accepted invalid threshold")
	}
}

func TestMeaningfulChangesAndCanonicalOrdering(t *testing.T) {
	observer, _ := NewObserver(time.Minute)
	attempt := observationAttempt(time.Unix(1000, 0))
	attempt.Result.Aircraft[0].Altitude = []byte(`0`)
	if _, changed, err := observer.Observe(attempt); err != nil || !changed {
		t.Fatal(err)
	}
	attempt.Result.Aircraft[0].Altitude = []byte(`0.0`)
	if _, changed, err := observer.Observe(attempt); err != nil || changed {
		t.Fatal("equivalent numeric altitude repeated")
	}
	attempt.Result.Aircraft[0].Altitude = []byte(`"ground"`)
	if _, changed, err := observer.Observe(attempt); err != nil || !changed {
		t.Fatal("ground change suppressed")
	}
	attempt.Result.Aircraft[0].Latitude = number(1)
	if _, changed, err := observer.Observe(attempt); err != nil || !changed {
		t.Fatal("position change suppressed")
	}
	second := attempt.Result.Aircraft[0]
	second.Hex = "000001"
	attempt.Result.Aircraft = append(attempt.Result.Aircraft, second)
	event, _, err := observer.Observe(attempt)
	if err != nil || event.Result.Aircraft[0].Hex != "000001" {
		t.Fatal("output not sorted")
	}
	attempt.Result.Aircraft[0], attempt.Result.Aircraft[1] = attempt.Result.Aircraft[1], attempt.Result.Aircraft[0]
	if _, changed, err := observer.Observe(attempt); err != nil || changed {
		t.Fatal("provider ordering repeated observation")
	}
}

func TestUnknownErrorsAndSeparateTargetState(t *testing.T) {
	observer, _ := NewObserver(time.Minute)
	attempt := observationAttempt(time.Now())
	attempt.Err = errors.New("unclassified source error")
	event, changed, err := observer.Observe(attempt)
	if err != nil || !changed || event.State != Unknown || event.Failure.Kind != "unknown" {
		t.Fatalf("unexpected failure: %+v", event)
	}
	attempt.Query.Identifier = "def456"
	if _, changed, err := observer.Observe(attempt); err != nil || !changed {
		t.Fatal("another target's event suppressed")
	}
}
