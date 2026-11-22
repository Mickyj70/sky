// Package flight resolves dated commercial flight legs from a provider.
// Broadcast callsigns and aircraft observations are separate concepts.
package flight

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "time/tzdata"
)

var (
	ErrInvalid     = errors.New("invalid flight data or query")
	ErrNoMatch     = errors.New("no matching flight for service date")
	ErrAmbiguous   = errors.New("multiple flight legs match")
	ErrIncomplete  = errors.New("flight identity is incomplete")
	ErrUnsupported = errors.New("provider does not support the requested lookup")
)

type Number struct {
	IATA string
	ICAO string
}

type Times struct {
	Scheduled *time.Time
	Estimated *time.Time
	Actual    *time.Time
}

type AirportEvent struct {
	Airport string
	Zone    string // IANA timezone if known
	Gate    Times
	Runway  Times
}

type Status string

const (
	Unknown   Status = ""
	Scheduled Status = "scheduled"
	Active    Status = "active"
	Landed    Status = "landed"
	Cancelled Status = "cancelled"
	Diverted  Status = "diverted"
)

type Leg struct {
	ServiceDate  string // Original scheduled departure date in the departure timezone.
	Operating    Number
	Marketing    []Number
	Departure    AirportEvent
	Arrival      AirportEvent
	Registration *string
	Status       Status
	RawStatus    string
	Provider     string
	Reference    string
	RetrievedAt  time.Time
}

// TimingConflict flags an inconsistent actual chronology without changing
// supplied timestamps or deriving status. Missing times are skipped.
func (leg Leg) TimingConflict() bool {
	var previous *time.Time
	for _, actual := range []*time.Time{leg.Departure.Gate.Actual, leg.Departure.Runway.Actual, leg.Arrival.Runway.Actual, leg.Arrival.Gate.Actual} {
		if actual == nil {
			continue
		}
		if previous != nil && actual.Before(*previous) {
			return true
		}
		previous = actual
	}
	return false
}

type Query struct {
	Number string // Fully qualified IATA or ICAO flight designator.
	Date   string // YYYY-MM-DD in the departure timezone.
	Origin string // Optional IATA or ICAO departure airport.
}

// Source returns candidate legs with provider provenance. Adapters should
// return ErrUnsupported for dates or lookup modes outside their entitlement.
type Source interface {
	Lookup(context.Context, Query) ([]Leg, error)
}

var designator = regexp.MustCompile(`^(?:[A-Z0-9]{2}|[A-Z]{3})[0-9]{1,4}[A-Z]?$`)
var airport = regexp.MustCompile(`^[A-Z]{3,4}$`)

func canonical(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

func validDate(date string) bool {
	t, err := time.Parse(time.DateOnly, date)
	return err == nil && t.Year() != 0 && t.Format(time.DateOnly) == date
}

func (n Number) matches(value string) bool {
	return canonical(n.IATA) == value || canonical(n.ICAO) == value
}

func (leg Leg) matches(number string) bool {
	if leg.Operating.matches(number) {
		return true
	}
	for _, alias := range leg.Marketing {
		if alias.matches(number) {
			return true
		}
	}
	return false
}

func (leg Leg) serviceDate() (string, error) {
	if leg.ServiceDate != "" {
		if !validDate(leg.ServiceDate) {
			return "", fmt.Errorf("%w: malformed service date", ErrInvalid)
		}
		return leg.ServiceDate, nil
	}
	if leg.Departure.Zone == "" || leg.Departure.Gate.Scheduled == nil {
		return "", ErrIncomplete
	}
	if leg.Departure.Zone == "Local" {
		return "", fmt.Errorf("%w: departure zone must be explicit", ErrInvalid)
	}
	zone, err := time.LoadLocation(leg.Departure.Zone)
	if err != nil {
		return "", fmt.Errorf("%w: invalid departure zone", ErrInvalid)
	}
	return leg.Departure.Gate.Scheduled.In(zone).Format(time.DateOnly), nil
}

// Resolve selects one leg. Multiple matches remain ambiguous even if they
// share a flight number. An undated matching candidate prevents a definitive
// match because it could be another leg on the requested day.
func Resolve(ctx context.Context, source Source, query Query) (Leg, error) {
	query.Number, query.Origin = canonical(query.Number), canonical(query.Origin)
	if source == nil {
		return Leg{}, ErrUnsupported
	}
	if !designator.MatchString(query.Number) || !validDate(query.Date) || (query.Origin != "" && !airport.MatchString(query.Origin)) {
		return Leg{}, ErrInvalid
	}
	legs, err := source.Lookup(ctx, query)
	if err != nil {
		return Leg{}, err
	}
	var match Leg
	found, incomplete := false, false
	for _, leg := range legs {
		if !leg.matches(query.Number) {
			continue
		}
		origin := canonical(leg.Departure.Airport)
		if query.Origin != "" && origin != "" && origin != query.Origin {
			continue
		}
		date, err := leg.serviceDate()
		if errors.Is(err, ErrIncomplete) {
			incomplete = true
			continue
		}
		if err != nil {
			return Leg{}, err
		}
		if date != query.Date {
			continue
		}
		if query.Origin != "" && origin == "" {
			incomplete = true
			continue
		}
		if found {
			return Leg{}, ErrAmbiguous
		}
		leg.ServiceDate = date
		match, found = leg, true
	}
	if incomplete {
		return Leg{}, ErrIncomplete
	}
	if !found {
		return Leg{}, ErrNoMatch
	}
	if strings.TrimSpace(match.Provider) == "" || match.RetrievedAt.IsZero() {
		return Leg{}, fmt.Errorf("%w: missing provider provenance", ErrIncomplete)
	}
	return match, nil
}
