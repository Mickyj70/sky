package provider

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ErrorKind string

const (
	RateLimited     ErrorKind = "rate_limited"
	Unavailable     ErrorKind = "unavailable"
	Unauthorized    ErrorKind = "unauthorized"
	Rejected        ErrorKind = "rejected"
	InvalidResponse ErrorKind = "invalid_response"
	Canceled        ErrorKind = "canceled"
	Timeout         ErrorKind = "timeout"
)

// Error exposes actionable failure information without parsing its message.
// RetryAt is an absolute UTC cooldown deadline when Retry-After is valid.
// Retryable does not authorize retries: callers must enforce their own budgets.
type Error struct {
	Kind       ErrorKind
	StatusCode int
	RetryAt    *time.Time
	Message    string
	Cause      error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }
func (e *Error) Retryable() bool {
	return e.Kind == RateLimited || e.Kind == Unavailable || e.Kind == Timeout
}

func retryAt(value string, now time.Time) *time.Time {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		// Guard duration overflow. An unrepresentable delay is unknown, not zero.
		if seconds < 0 || seconds > int64((1<<63-1)/int64(time.Second)) {
			return nil
		}
		t := now.Add(time.Duration(seconds) * time.Second).UTC()
		return &t
	}
	t, err := http.ParseTime(value)
	if err != nil {
		return nil
	}
	if t.Before(now) {
		t = now
	}
	t = t.UTC()
	return &t
}
