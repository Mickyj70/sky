package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Mickyj70/sky/internal/provider"
	"github.com/Mickyj70/sky/internal/storage"
)

type watchClock struct {
	now    time.Time
	onWait func()
}

func (c *watchClock) Now() time.Time { return c.now }
func (c *watchClock) Wait(ctx context.Context, d time.Duration) error {
	if c.onWait != nil {
		c.onWait()
	}
	c.now = c.now.Add(d)
	return ctx.Err()
}
func prepareWatch(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	s, err := storage.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, _, err := s.Add(context.Background(), storage.Registration, "G-TEST"); err != nil {
		t.Fatal(err)
	}
	return dir
}
func decodeEvents(t *testing.T, out []byte) []watchEvent {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(out))
	events := []watchEvent{}
	for {
		var event watchEvent
		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			return events
		}
		if err != nil {
			t.Fatalf("not pure NDJSON: %v\n%s", err, out)
		}
		events = append(events, event)
	}
}

func TestWatchRunBudgetAndNDJSON(t *testing.T) {
	dir := prepareWatch(t)
	clock := &watchClock{now: time.Unix(1000, 0).UTC()}
	calls := 0
	source := sourceFunc(func(ctx context.Context, q provider.Query) (provider.Result, error) {
		calls++
		if q.Kind != provider.ByRegistration || q.Identifier != "G-TEST" {
			t.Fatalf("wrong query: %+v", q)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Error("request has no timeout")
		}
		zero := 0.0
		snapshot := clock.now
		return provider.Result{Source: "test", SnapshotAt: &snapshot, RetrievedAt: clock.now, Aircraft: []provider.Aircraft{{Hex: "abc123", Latitude: &zero, Longitude: &zero, PositionSeen: &zero}}}, nil
	})
	command := newWithClock(source, clock)
	command.SetArgs([]string{"watch", "run", "--data-dir", dir, "--interval", "5m", "--max-requests", "3", "--json"})
	var out, stderr bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&stderr)
	if err := Execute(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	events := decodeEvents(t, out.Bytes())
	observations, progress := 0, 0
	for _, event := range events {
		if event.Type == "observation" {
			observations++
		}
		if event.Type == "progress" {
			progress++
			if event.RequestsRemaining != 3-progress {
				t.Fatal("wrong remaining budget")
			}
		}
	}
	if calls != 3 || observations != 1 || progress != 3 || stderr.Len() != 0 {
		t.Fatalf("calls=%d observations=%d progress=%d stderr=%s", calls, observations, progress, stderr.String())
	}
	if events[0].Type != "started" || events[len(events)-1].Type != "stopped" || events[len(events)-1].Reason != "budget_exhausted" {
		t.Fatal("missing lifecycle events")
	}
}

func TestWatchRunMissingPlainText(t *testing.T) {
	source := sourceFunc(func(context.Context, provider.Query) (provider.Result, error) {
		return provider.Result{Source: "test", Aircraft: []provider.Aircraft{}}, nil
	})
	command := newWithClock(source, &watchClock{now: time.Now()})
	command.SetArgs([]string{"watch", "run", "--data-dir", prepareWatch(t), "--max-requests", "1"})
	var out bytes.Buffer
	command.SetOut(&out)
	if err := Execute(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{"missing", "does not establish flight status", "remaining: 0", "budget_exhausted", "process exits"} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q in %s", expected, text)
		}
	}
	if strings.Contains(strings.ToLower(text), "landed") {
		t.Fatal("missing data inferred a landing")
	}
}

func TestWatchRunRecoverableAndTerminalProviderErrors(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		t.Run(map[bool]string{false: "outage", true: "authorization"}[terminal], func(t *testing.T) {
			calls := 0
			source := sourceFunc(func(context.Context, provider.Query) (provider.Result, error) {
				calls++
				kind := provider.Unavailable
				if terminal {
					kind = provider.Unauthorized
				}
				return provider.Result{}, &provider.Error{Kind: kind, Message: "provider failure"}
			})
			command := newWithClock(source, &watchClock{now: time.Now()})
			command.SetArgs([]string{"watch", "run", "--data-dir", prepareWatch(t), "--max-requests", "2", "--json"})
			var out bytes.Buffer
			command.SetOut(&out)
			err := Execute(context.Background(), command)
			events := decodeEvents(t, out.Bytes())
			last := events[len(events)-1]
			if terminal {
				if ExitCode(err) != 5 || calls != 1 || last.Reason != "provider_error" {
					t.Fatalf("terminal: %v %d %+v", err, calls, last)
				}
			} else if err != nil || calls != 2 || last.Reason != "budget_exhausted" {
				t.Fatalf("retryable: %v %d %+v", err, calls, last)
			}
			if events[1].Type != "provider_error" || events[1].Observation.State != "unknown" {
				t.Fatal("outage not represented as unknown")
			}
		})
	}
}

func TestWatchRunCancellationAccuratelyCountsInFlightAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := sourceFunc(func(context.Context, provider.Query) (provider.Result, error) {
		cancel()
		return provider.Result{}, context.Canceled
	})
	command := newWithClock(source, &watchClock{now: time.Now()})
	command.SetArgs([]string{"watch", "run", "--data-dir", prepareWatch(t), "--max-requests", "5", "--json"})
	var out bytes.Buffer
	command.SetOut(&out)
	err := Execute(ctx, command)
	if ExitCode(err) != 130 {
		t.Fatalf("expected cancellation, got %v", err)
	}
	events := decodeEvents(t, out.Bytes())
	last := events[len(events)-1]
	if last.Reason != "canceled" || last.RequestsUsed != 1 || last.RequestsRemaining != 4 {
		t.Fatalf("incorrect canceled accounting: %+v", last)
	}
}

func TestWatchRunValidationAndOutputFailure(t *testing.T) {
	dir := prepareWatch(t)
	calls := 0
	source := sourceFunc(func(context.Context, provider.Query) (provider.Result, error) { calls++; return provider.Result{}, nil })
	for _, args := range [][]string{{"--interval", "0s", "--max-requests", "1"}, {"--max-requests", "0"}, {"--max-requests", "1", "--stale-after", "0s"}, {}} {
		command := newWithClock(source, &watchClock{now: time.Now()})
		command.SetArgs(append([]string{"watch", "run", "--data-dir", dir}, args...))
		command.SetOut(&bytes.Buffer{})
		if err := Execute(context.Background(), command); ExitCode(err) != 2 {
			t.Fatalf("invalid args accepted: %v %v", args, err)
		}
	}
	command := newWithClock(source, &watchClock{now: time.Now()})
	command.SetArgs([]string{"watch", "run", "--data-dir", dir, "--max-requests", "1"})
	command.SetOut(brokenWriter{})
	if err := Execute(context.Background(), command); ExitCode(err) != 1 {
		t.Fatalf("expected output failure, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("invalid/start-output-failed run made %d requests", calls)
	}
	command = newWithClock(source, &watchClock{now: time.Now()})
	command.SetArgs([]string{"watch", "run", "--data-dir", t.TempDir(), "--max-requests", "1"})
	command.SetOut(&bytes.Buffer{})
	if err := Execute(context.Background(), command); ExitCode(err) != 2 {
		t.Fatal("empty watchlist accepted")
	}
}

type failAfterStarted struct{ writes int }

func (w *failAfterStarted) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == 1 {
		return len(p), nil
	}
	return 0, io.ErrClosedPipe
}
func TestWatchRunStopsRequestsAfterOutputBreaks(t *testing.T) {
	calls := 0
	source := sourceFunc(func(context.Context, provider.Query) (provider.Result, error) {
		calls++
		return provider.Result{Aircraft: []provider.Aircraft{}}, nil
	})
	command := newWithClock(source, &watchClock{now: time.Now()})
	command.SetArgs([]string{"watch", "run", "--data-dir", prepareWatch(t), "--max-requests", "5", "--json"})
	writer := &failAfterStarted{}
	command.SetOut(writer)
	err := Execute(context.Background(), command)
	if !errors.Is(err, io.ErrClosedPipe) || ExitCode(err) != 1 || calls != 1 || writer.writes != 2 {
		t.Fatalf("output failure: %v calls=%d writes=%d", err, calls, writer.writes)
	}
}
