// Package tracking schedules bounded, sequential lookups for saved aircraft.
package tracking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Mickyj70/sky/internal/provider"
	"github.com/Mickyj70/sky/internal/storage"
)

const (
	initialBackoff = time.Second
	maxBackoff     = time.Minute
)

// Clock allows polling and cooldowns to be tested without wall-clock delays.
type Clock interface {
	Now() time.Time
	Wait(context.Context, time.Duration) error
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) Wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type Config struct {
	// Interval is the minimum time between the starts of successive cycles.
	Interval    time.Duration
	MaxRequests int
}

type Budget struct {
	RequestsPerCycle  int
	RequestsRemaining int
}

type Attempt struct {
	At     time.Time
	Query  provider.Query
	Result provider.Result
	Err    error
	Budget Budget
}

type Scheduler struct {
	source  provider.Source
	clock   Clock
	config  Config
	queries []provider.Query
}

// New validates a snapshot of the saved targets and deduplicates normalized
// identifiers in input order. A nil clock uses wall time.
func New(source provider.Source, clock Clock, config Config, targets []storage.Target) (*Scheduler, error) {
	if source == nil {
		return nil, errors.New("watch requires a provider")
	}
	if config.Interval <= 0 || config.MaxRequests <= 0 {
		return nil, errors.New("watch interval and request budget must be positive")
	}
	if len(targets) == 0 {
		return nil, errors.New("watch requires at least one saved target")
	}
	if clock == nil {
		clock = systemClock{}
	}

	seen := make(map[provider.Query]struct{}, len(targets))
	queries := make([]provider.Query, 0, len(targets))
	for _, target := range targets {
		id, err := storage.Normalize(target.Kind, target.Identifier)
		if err != nil {
			return nil, err
		}
		kind := provider.ByRegistration
		if target.Kind == storage.Hex {
			kind = provider.ByHex
		}
		query := provider.Query{Kind: kind, Identifier: id}
		if _, exists := seen[query]; !exists {
			seen[query] = struct{}{}
			queries = append(queries, query)
		}
	}

	return &Scheduler{source: source, clock: clock, config: config, queries: queries}, nil
}

// Plan returns the nominal full-cycle cost and starting budget for a run.
func (s *Scheduler) Plan() Budget {
	return Budget{RequestsPerCycle: len(s.queries), RequestsRemaining: s.config.MaxRequests}
}

// Run spends one budget unit per lookup attempt, including failures. Transient
// failures delay all subsequent lookups and resume with the next target in order.
// Run returns nil when the budget is exhausted. The observer may stop the run
// by returning an error, for example when writing output fails.
func (s *Scheduler) Run(ctx context.Context, observe func(Attempt) error) error {
	budget := s.Plan()
	var cycleDue, cooldown time.Time
	var backoff time.Duration
	for index := 0; budget.RequestsRemaining > 0; index = (index + 1) % len(s.queries) {
		due := cooldown
		if index == 0 && cycleDue.After(due) {
			due = cycleDue
		}
		if delay := due.Sub(s.clock.Now()); delay > 0 {
			if err := s.clock.Wait(ctx, delay); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if index == 0 {
			cycleDue = s.clock.Now().Add(s.config.Interval)
		}

		query := s.queries[index]
		budget.RequestsRemaining--
		result, lookupErr := s.source.Lookup(ctx, query)
		if err := ctx.Err(); err != nil {
			return err
		}

		var failure *provider.Error
		retryable := errors.As(lookupErr, &failure) && failure.Retryable()
		if retryable {
			backoff = min(max(initialBackoff, 2*backoff), maxBackoff)
			cooldown = s.clock.Now().Add(backoff)
			if failure.RetryAt != nil && failure.RetryAt.After(cooldown) {
				cooldown = *failure.RetryAt
			}
		} else {
			backoff = 0
			cooldown = time.Time{}
		}

		if observe != nil {
			if err := observe(Attempt{At: s.clock.Now().UTC(), Query: query, Result: result, Err: lookupErr, Budget: budget}); err != nil {
				return err
			}
		}
		if lookupErr != nil && !retryable {
			return fmt.Errorf("watch %s %s: %w", query.Kind, query.Identifier, lookupErr)
		}
	}
	return ctx.Err()
}
