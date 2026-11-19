package tracking

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Mickyj70/sky/internal/provider"
	"github.com/Mickyj70/sky/internal/storage"
)

type fakeClock struct {
	now    time.Time
	onWait func()
}

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) Wait(ctx context.Context, delay time.Duration) error {
	if c.onWait != nil {
		c.onWait()
	}
	c.now = c.now.Add(delay)
	return ctx.Err()
}

type lookupFunc func(context.Context, provider.Query) (provider.Result, error)

func (f lookupFunc) Lookup(ctx context.Context, q provider.Query) (provider.Result, error) {
	return f(ctx, q)
}

func TestBudgetAndCooldown(t *testing.T) {
	start := time.Unix(100, 0)
	clock := &fakeClock{now: start}
	retryAt := start.Add(30 * time.Second)
	var queries []provider.Query
	var offsets []time.Duration
	var remaining []int
	source := lookupFunc(func(_ context.Context, q provider.Query) (provider.Result, error) {
		queries = append(queries, q)
		offsets = append(offsets, clock.Now().Sub(start))
		switch len(queries) {
		case 1:
			return provider.Result{}, &provider.Error{Kind: provider.RateLimited, RetryAt: &retryAt}
		case 2:
			return provider.Result{}, &provider.Error{Kind: provider.Unavailable}
		default:
			return provider.Result{}, nil
		}
	})
	s, err := New(source, clock, Config{Interval: 10 * time.Second, MaxRequests: 5}, []storage.Target{
		{Kind: storage.Registration, Identifier: " g-vbel "},
		{Kind: storage.Registration, Identifier: "G-VBEL"},
		{Kind: storage.Hex, Identifier: "4073CE"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Plan(); got != (Budget{RequestsPerCycle: 2, RequestsRemaining: 5}) {
		t.Fatalf("plan = %+v", got)
	}
	err = s.Run(context.Background(), func(a Attempt) error {
		remaining = append(remaining, a.Budget.RequestsRemaining)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	reg := provider.Query{Kind: provider.ByRegistration, Identifier: "G-VBEL"}
	hex := provider.Query{Kind: provider.ByHex, Identifier: "4073ce"}
	if want := []provider.Query{reg, hex, reg, hex, reg}; !reflect.DeepEqual(queries, want) {
		t.Fatalf("queries = %v, want %v", queries, want)
	}
	if want := []time.Duration{0, 30 * time.Second, 32 * time.Second, 32 * time.Second, 42 * time.Second}; !reflect.DeepEqual(offsets, want) {
		t.Fatalf("offsets = %v, want %v", offsets, want)
	}
	if want := []int{4, 3, 2, 1, 0}; !reflect.DeepEqual(remaining, want) {
		t.Fatalf("remaining = %v, want %v", remaining, want)
	}
}

func TestBoundedBackoffAndReset(t *testing.T) {
	clock := &fakeClock{now: time.Unix(100, 0)}
	var offsets []time.Duration
	source := lookupFunc(func(_ context.Context, _ provider.Query) (provider.Result, error) {
		offsets = append(offsets, clock.Now().Sub(time.Unix(100, 0)))
		if len(offsets) == 9 {
			return provider.Result{}, nil
		}
		return provider.Result{}, &provider.Error{Kind: provider.RateLimited}
	})
	s, err := New(source, clock, Config{Interval: time.Second, MaxRequests: 11}, []storage.Target{
		{Kind: storage.Hex, Identifier: "4073ce"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Run(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	wantSeconds := []int{0, 1, 3, 7, 15, 31, 63, 123, 183, 184, 185}
	for index, seconds := range wantSeconds {
		if got := offsets[index]; got != time.Duration(seconds)*time.Second {
			t.Errorf("attempt %d at %s, want %ds", index+1, got, seconds)
		}
	}
}

func TestStops(t *testing.T) {
	for _, name := range []string{"pre-canceled", "cooldown cancellation", "lookup cancellation", "terminal failure", "observer failure"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			clock := &fakeClock{now: time.Unix(100, 0)}
			want := error(context.Canceled)
			var calls, events int
			fatal := &provider.Error{Kind: provider.Unauthorized}
			outputErr := errors.New("output closed")
			switch name {
			case "pre-canceled":
				cancel()
			case "cooldown cancellation":
				clock.onWait = cancel
			case "terminal failure":
				want = fatal
			case "observer failure":
				want = outputErr
			}
			source := lookupFunc(func(lookupCtx context.Context, _ provider.Query) (provider.Result, error) {
				calls++
				switch name {
				case "lookup cancellation":
					cancel()
					return provider.Result{}, lookupCtx.Err()
				case "terminal failure":
					return provider.Result{}, fatal
				default:
					return provider.Result{}, &provider.Error{Kind: provider.RateLimited}
				}
			})
			s, err := New(source, clock, Config{Interval: time.Second, MaxRequests: 10}, []storage.Target{
				{Kind: storage.Hex, Identifier: "4073ce"},
			})
			if err != nil {
				t.Fatal(err)
			}
			err = s.Run(ctx, func(Attempt) error {
				events++
				if name == "observer failure" {
					return outputErr
				}
				return nil
			})
			wantCalls, wantEvents := 1, 1
			switch name {
			case "pre-canceled":
				wantCalls, wantEvents = 0, 0
			case "lookup cancellation":
				wantEvents = 0
			}
			if !errors.Is(err, want) || calls != wantCalls || events != wantEvents {
				t.Fatalf("error = %v, calls = %d, events = %d; want %v, %d, %d", err, calls, events, want, wantCalls, wantEvents)
			}
		})
	}
}

func TestRejectsInvalidPlan(t *testing.T) {
	source := lookupFunc(func(context.Context, provider.Query) (provider.Result, error) {
		t.Fatal("planning made a provider call")
		return provider.Result{}, nil
	})
	valid := []storage.Target{{Kind: storage.Hex, Identifier: "4073ce"}}
	for _, tc := range []struct {
		config  Config
		targets []storage.Target
	}{
		{Config{Interval: time.Second}, valid},
		{Config{MaxRequests: 1}, valid},
		{Config{Interval: time.Second, MaxRequests: -1}, valid},
		{Config{Interval: time.Second, MaxRequests: 1}, nil},
		{Config{Interval: time.Second, MaxRequests: 1}, []storage.Target{{Kind: storage.Hex, Identifier: "invalid"}}},
	} {
		if _, err := New(source, nil, tc.config, tc.targets); err == nil {
			t.Errorf("accepted invalid plan %+v", tc)
		}
	}
}
