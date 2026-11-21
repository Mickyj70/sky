package flight

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fixtureSource []Leg

func (s fixtureSource) Lookup(context.Context, Query) ([]Leg, error) { return s, nil }

func instant(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &t
}

func TestResolveDatedCodeshareAndLegs(t *testing.T) {
	registration := "N123AB"
	leg := Leg{
		Operating: Number{IATA: "ZZ123", ICAO: "ZZZ123"},
		Marketing: []Number{{IATA: "YY456"}},
		Departure: AirportEvent{Airport: "JFK", Zone: "America/New_York",
			Gate: Times{Scheduled: instant("2026-11-01T03:30:00Z")}},
		Registration: &registration, Provider: "synthetic", Reference: "leg-1",
		RetrievedAt: *instant("2026-11-01T09:00:00Z"),
	}
	next := leg
	next.Reference, next.Registration = "leg-2", nil
	next.ServiceDate, next.Status, next.RawStatus = "2026-11-01", Cancelled, "CXL"
	other := next
	other.Reference, other.Departure.Airport = "leg-3", "BOS"
	unknown := next
	unknown.ServiceDate, unknown.Departure.Zone = "", ""
	zoneAmbiguous := leg
	zoneAmbiguous.Departure.Gate.Scheduled = instant("2026-11-01T06:30:00Z")
	zoneAmbiguous.Reference = "second DST hour"
	for _, tc := range []struct {
		name   string
		source fixtureSource
		query  Query
		ref    string
		want   error
	}{
		{"codeshare across UTC midnight", fixtureSource{next, leg}, Query{" yy456 ", "2026-10-31", "jfk"}, "leg-1", nil},
		{"reused number and cancelled flight", fixtureSource{leg, next}, Query{"ZZZ123", "2026-11-01", ""}, "leg-2", nil},
		{"multiple legs", fixtureSource{next, other}, Query{"ZZ123", "2026-11-01", ""}, "", ErrAmbiguous},
		{"origin disambiguates", fixtureSource{next, other}, Query{"ZZ123", "2026-11-01", "BOS"}, "leg-3", nil},
		{"missing zone", fixtureSource{next, unknown}, Query{"ZZ123", "2026-11-01", ""}, "", ErrIncomplete},
		{"DST repeated hour uses actual offset", fixtureSource{zoneAmbiguous}, Query{"ZZ123", "2026-11-01", ""}, "second DST hour", nil},
		{"missing date match", fixtureSource{leg}, Query{"ZZ123", "2026-11-02", ""}, "", ErrNoMatch},
		{"invalid date", fixtureSource{leg}, Query{"ZZ123", "2026-02-30", ""}, "", ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve(context.Background(), tc.source, tc.query)
			if !errors.Is(err, tc.want) || got.Reference != tc.ref {
				t.Fatalf("reference=%q error=%v", got.Reference, err)
			}
			if tc.ref == "leg-2" && (got.Registration != nil || got.Status != Cancelled || got.RawStatus != "CXL") {
				t.Fatalf("cancellation or missing registration changed: %+v", got)
			}
		})
	}
}

func TestTimingConflictPreservesProviderData(t *testing.T) {
	gate := instant("2026-11-21T11:00:00Z")
	runway := instant("2026-11-21T10:00:00Z")
	leg := Leg{Departure: AirportEvent{Gate: Times{Actual: gate}, Runway: Times{Actual: runway}}}
	if !leg.TimingConflict() || leg.Status != Unknown || leg.Departure.Gate.Actual != gate {
		t.Fatal("expected conflict with unchanged status and timestamps")
	}
	leg.Departure.Gate.Actual = nil
	if leg.TimingConflict() {
		t.Fatal("missing gate time is not a conflict")
	}
}
