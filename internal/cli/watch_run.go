package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mickyj70/sky/internal/history"
	"github.com/Mickyj70/sky/internal/provider"
	"github.com/Mickyj70/sky/internal/storage"
	"github.com/Mickyj70/sky/internal/tracking"
	"github.com/spf13/cobra"
)

type timedSource struct {
	source    provider.Source
	timeout   time.Duration
	onAttempt func()
}

func (s timedSource) Lookup(ctx context.Context, q provider.Query) (provider.Result, error) {
	if s.onAttempt != nil {
		s.onAttempt()
	}
	request, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.source.Lookup(request, q)
}

type watchEvent struct {
	Type                    string                `json:"type"`
	At                      time.Time             `json:"at"`
	Observation             *tracking.Observation `json:"observation,omitempty"`
	RequestsPerCycle        int                   `json:"requests_per_cycle"`
	RequestsRemaining       int                   `json:"requests_remaining"`
	RequestsUsed            int                   `json:"requests_used"`
	IntervalSeconds         float64               `json:"interval_seconds,omitempty"`
	StaleAfterSeconds       float64               `json:"stale_after_seconds,omitempty"`
	HistoryDatabase         string                `json:"history_database,omitempty"`
	HistoryRetentionSeconds float64               `json:"history_retention_seconds,omitempty"`
	Reason                  string                `json:"reason,omitempty"`
}

func watchRunCommand(asJSON *bool, source provider.Source, timeout *time.Duration, clock tracking.Clock, open func(*cobra.Command) (*storage.Store, error)) *cobra.Command {
	var interval, staleAfter, retention time.Duration
	var record bool
	var maxRequests int
	command := &cobra.Command{Use: "run", Short: "Poll saved targets with a finite request budget; stop when this process exits", Args: cobra.NoArgs,
		Example: "  sky watch run --interval 5m --max-requests 100\n  sky watch run --interval 5m --max-requests 10 --stale-after 1m --json\n  sky watch run --interval 5m --max-requests 10 --record --history-retention 168h",
		RunE: func(cmd *cobra.Command, args []string) error {
			if interval <= 0 || maxRequests <= 0 || staleAfter <= 0 || *timeout <= 0 {
				return fmt.Errorf("interval, max-requests, stale-after and timeout must be positive")
			}
			if retention <= 0 {
				return fmt.Errorf("history-retention must be positive")
			}
			if cmd.Flags().Changed("history-retention") && !record {
				return fmt.Errorf("history-retention requires --record")
			}
			if source == nil {
				return &executionError{fmt.Errorf("watch requires a provider")}
			}
			store, err := open(cmd)
			if err != nil {
				return err
			}
			dataDir := filepath.Dir(store.Path())
			saved, readErr := store.List(cmd.Context())
			closeErr := store.Close()
			if readErr != nil {
				return &executionError{readErr}
			}
			if closeErr != nil {
				return &executionError{closeErr}
			}
			if len(saved) == 0 {
				return fmt.Errorf("watchlist is empty; add a registration or hex target first")
			}
			used := 0
			scheduler, err := tracking.New(timedSource{source: source, timeout: *timeout, onAttempt: func() { used++ }}, clock, tracking.Config{Interval: interval, MaxRequests: maxRequests}, saved)
			if err != nil {
				return err
			}
			observer, err := tracking.NewObserver(staleAfter)
			if err != nil {
				return err
			}
			var archive *history.Store
			if record {
				archive, err = history.Open(cmd.Context(), dataDir)
				if err != nil {
					return &executionError{err}
				}
				defer archive.Close()
			}
			budget := scheduler.Plan()
			now := func() time.Time {
				if clock != nil {
					return clock.Now().UTC()
				}
				return time.Now().UTC()
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			write := func(event watchEvent) error {
				if *asJSON {
					return encoder.Encode(event)
				}
				return renderWatchEvent(cmd, event)
			}
			event := func(kind string, at time.Time) watchEvent {
				return watchEvent{Type: kind, At: at, RequestsPerCycle: budget.RequestsPerCycle, RequestsRemaining: budget.RequestsRemaining, RequestsUsed: maxRequests - budget.RequestsRemaining}
			}
			started := event("started", now())
			if archive != nil {
				started.HistoryDatabase = archive.Path()
				started.HistoryRetentionSeconds = retention.Seconds()
			}
			started.IntervalSeconds = interval.Seconds()
			started.StaleAfterSeconds = staleAfter.Seconds()
			if err := write(started); err != nil {
				return &executionError{err}
			}
			outputFailed := false
			err = scheduler.Run(cmd.Context(), func(attempt tracking.Attempt) error {
				budget = attempt.Budget
				observation, changed, err := observer.Observe(attempt)
				if err != nil {
					return err
				}
				if archive != nil && observation.Failure == nil {
					if _, err := archive.Record(cmd.Context(), observation, retention); err != nil {
						return fmt.Errorf("record history: %w", err)
					}
				}
				if changed {
					kind := "observation"
					if observation.Failure != nil {
						kind = "provider_error"
					}
					update := event(kind, attempt.At)
					update.Observation = &observation
					if err := write(update); err != nil {
						outputFailed = true
						return err
					}
				}
				if err := write(event("progress", attempt.At)); err != nil {
					outputFailed = true
					return err
				}
				return nil
			})
			if outputFailed {
				return &executionError{err}
			}
			budget.RequestsRemaining = maxRequests - used
			stopped := event("stopped", now())
			stopped.Reason = "budget_exhausted"
			if err != nil {
				stopped.Reason = "error"
				if errors.Is(err, context.Canceled) {
					stopped.Reason = "canceled"
				} else if errors.Is(err, context.DeadlineExceeded) {
					stopped.Reason = "deadline"
				} else {
					var failure *provider.Error
					if errors.As(err, &failure) {
						stopped.Reason = "provider_error"
					}
				}
			}
			if writeErr := write(stopped); writeErr != nil {
				return &executionError{writeErr}
			}
			if err != nil {
				return &executionError{err}
			}
			return nil
		}}
	command.Flags().BoolVar(&record, "record", false, "Save successful lookup snapshots in local history.db")
	command.Flags().DurationVar(&retention, "history-retention", 30*24*time.Hour, "Keep first local snapshot encounters for this duration while recording")
	command.Flags().DurationVar(&interval, "interval", 5*time.Minute, "Minimum interval between cycle starts")
	command.Flags().IntVar(&maxRequests, "max-requests", 0, "Required per-run lookup budget, including failed attempts")
	command.Flags().DurationVar(&staleAfter, "stale-after", time.Minute, "Position age threshold, evaluated at each completed lookup")
	_ = command.MarkFlagRequired("max-requests")
	return command
}

func renderWatchEvent(cmd *cobra.Command, event watchEvent) error {
	out := cmd.OutOrStdout()
	switch event.Type {
	case "started":
		if _, err := fmt.Fprintf(out, "Watching %d unique target(s), %d requests per cycle. Budget: %d; interval: %gs; stale after: %gs.\nTargets are loaded once. Tracking stops when this process exits.\n", event.RequestsPerCycle, event.RequestsPerCycle, event.RequestsRemaining, event.IntervalSeconds, event.StaleAfterSeconds); err != nil {
			return err
		}
		if event.HistoryDatabase != "" {
			_, err := fmt.Fprintf(out, "Recording successful snapshots: %s; retention: %gs.\n", event.HistoryDatabase, event.HistoryRetentionSeconds)
			return err
		}
		return nil
	case "observation", "provider_error":
		observation := event.Observation
		target := fmt.Sprintf("%s %s", observation.Target.Kind, observation.Target.Identifier)
		if observation.Failure != nil {
			_, err := fmt.Fprintf(out, "%s %s unknown: provider %s: %s\n", event.At.Format(time.RFC3339), target, observation.Failure.Kind, terminalText(observation.Failure.Message))
			return err
		}
		if _, err := fmt.Fprintf(out, "%s %s %s\n", event.At.Format(time.RFC3339), target, observation.State); err != nil {
			return err
		}
		if observation.PositionAgeSeconds != nil {
			if _, err := fmt.Fprintf(out, "Best known position age: %.1fs\n", *observation.PositionAgeSeconds); err != nil {
				return err
			}
		}
		return render(out, *observation.Result)
	case "progress":
		_, err := fmt.Fprintf(out, "Requests used: %d; remaining: %d\n", event.RequestsUsed, event.RequestsRemaining)
		return err
	case "stopped":
		_, err := fmt.Fprintf(out, "Stopped: %s. Requests used: %d; remaining: %d\n", event.Reason, event.RequestsUsed, event.RequestsRemaining)
		return err
	}
	return nil
}
func terminalText(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, value)
}
