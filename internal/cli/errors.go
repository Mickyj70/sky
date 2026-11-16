package cli

import (
	"context"
	"errors"

	"github.com/Mickyj70/sky/internal/provider"
	"github.com/spf13/cobra"
)

type executionError struct{ error }

func (e *executionError) Unwrap() error { return e.error }

type usageError struct{ error }

func (e *usageError) Unwrap() error { return e.error }

// Execute classifies Cobra parsing/validation errors independently of runtime
// failures. Embedders that need the exit-code contract should use this wrapper.
func Execute(ctx context.Context, cmd *cobra.Command) error {
	err := cmd.ExecuteContext(ctx)
	if err == nil {
		return nil
	}
	var runtime *executionError
	if errors.As(err, &runtime) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return &usageError{err}
}

// ExitCode is the public process contract. Error strings are for people;
// provider.Error and errors.Is provide programmatic failure information.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return 124
	}
	var upstream *provider.Error
	if errors.As(err, &upstream) {
		switch upstream.Kind {
		case provider.Canceled:
			return 130
		case provider.Timeout:
			return 124
		case provider.RateLimited:
			return 4
		case provider.Unauthorized, provider.Rejected:
			return 5
		case provider.InvalidResponse:
			return 6
		default:
			return 3
		}
	}
	var usage *usageError
	if errors.As(err, &usage) {
		return 2
	}
	return 1
}
