package tracking

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/Mickyj70/sky/internal/provider"
)

type State string

const (
	Observed State = "observed"
	Stale    State = "stale"
	Missing  State = "missing"
	Unknown  State = "unknown"
)

type Target struct {
	Kind       provider.QueryKind `json:"kind"`
	Identifier string             `json:"identifier"`
}

type Failure struct {
	Kind       string     `json:"kind"`
	Message    string     `json:"message"`
	StatusCode int        `json:"status_code,omitempty"`
	Retryable  bool       `json:"retryable"`
	RetryAt    *time.Time `json:"retry_at,omitempty"`
}

type Observation struct {
	At                 time.Time        `json:"at"`
	Target             Target           `json:"target"`
	State              State            `json:"state"`
	PositionAgeSeconds *float64         `json:"position_age_seconds,omitempty"`
	Result             *provider.Result `json:"result,omitempty"`
	Failure            *Failure         `json:"failure,omitempty"`
}

// Observer classifies each completed lookup at its scheduler timestamp. It emits
// initial observations, meaningful data changes, state transitions and changed
// failures. Sampling timestamps/ages alone do not repeat an observation.
// It is owned by one scheduler run and is not used concurrently.
type Observer struct {
	staleAfter time.Duration
	signatures map[Target]string
}

func NewObserver(staleAfter time.Duration) (*Observer, error) {
	if staleAfter <= 0 {
		return nil, fmt.Errorf("stale-after must be positive")
	}
	return &Observer{staleAfter: staleAfter, signatures: make(map[Target]string)}, nil
}

func (o *Observer) Observe(attempt Attempt) (Observation, bool, error) {
	observation := Observation{At: attempt.At.UTC(), Target: Target{attempt.Query.Kind, attempt.Query.Identifier}, State: Unknown}
	if attempt.Err != nil {
		failure := &Failure{Kind: "unknown", Message: attempt.Err.Error()}
		var upstream *provider.Error
		if errors.As(attempt.Err, &upstream) {
			failure.Kind = string(upstream.Kind)
			failure.StatusCode = upstream.StatusCode
			failure.Retryable = upstream.Retryable()
			if upstream.RetryAt != nil {
				deadline := upstream.RetryAt.UTC()
				failure.RetryAt = &deadline
			}
		}
		observation.Failure = failure
	} else {
		result := attempt.Result
		result.Aircraft = append([]provider.Aircraft{}, result.Aircraft...)
		sort.SliceStable(result.Aircraft, func(i, j int) bool { return result.Aircraft[i].Hex < result.Aircraft[j].Hex })
		observation.Result = &result
		observation.State, observation.PositionAgeSeconds = classify(result, attempt.At, o.staleAfter)
	}
	signature, err := fingerprint(observation)
	if err != nil {
		return observation, false, err
	}
	previous, exists := o.signatures[observation.Target]
	o.signatures[observation.Target] = signature
	return observation, !exists || signature != previous, nil
}

func classify(result provider.Result, at time.Time, staleAfter time.Duration) (State, *float64) {
	if len(result.Aircraft) == 0 {
		return Missing, nil
	}
	if result.SnapshotAt == nil {
		return Unknown, nil
	}
	snapshotAge := math.Max(0, at.Sub(*result.SnapshotAt).Seconds())
	var best *float64
	for _, aircraft := range result.Aircraft {
		if aircraft.Latitude == nil || aircraft.Longitude == nil || aircraft.PositionSeen == nil {
			continue
		}
		age := snapshotAge + *aircraft.PositionSeen
		if best == nil || age < *best {
			v := age
			best = &v
		}
	}
	if best == nil {
		return Unknown, nil
	}
	if *best > staleAfter.Seconds() {
		return Stale, best
	}
	return Observed, best
}

func fingerprint(observation Observation) (string, error) {
	// Public output retains the provider contract. Deduplication ignores changing
	// retrieval/snapshot times and sampling ages, but includes ground/altitude,
	// position, speed, track, identifiers, state and provider/error metadata.
	var aircraft []provider.Aircraft
	var source string
	if observation.Result != nil {
		source = observation.Result.Source
		aircraft = append([]provider.Aircraft{}, observation.Result.Aircraft...)
		for i := range aircraft {
			aircraft[i].Seen = nil
			aircraft[i].PositionSeen = nil
			if len(aircraft[i].Altitude) > 0 {
				var value any
				if err := json.Unmarshal(aircraft[i].Altitude, &value); err != nil {
					return "", err
				}
				encoded, err := json.Marshal(value)
				if err != nil {
					return "", err
				}
				aircraft[i].Altitude = encoded
			}
		}
	}
	data, err := json.Marshal(struct {
		State    State
		Source   string
		Aircraft []provider.Aircraft
		Failure  *Failure
	}{observation.State, source, aircraft, observation.Failure})
	return string(data), err
}
