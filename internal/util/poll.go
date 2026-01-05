package util

import (
    "context"
    "fmt"
    "math"
    "time"
)

// PollFunc is the function to be executed by the Poll function.
// It should return an error to indicate a failure and that it should be retried.
// A nil error indicates success.
type PollFunc func() error

// PollOptions configures the behavior of the Poll function.
type PollOptions struct {
	// Tries is the maximum number of times to try the function.
	// If Tries is 0, it will retry until Timeout is reached.
	Tries int
	// Delay is the time to wait between retries.
	// If Delay is 0, binary exponential backoff is used, starting at 2 seconds.
	Delay time.Duration
	// Timeout is the maximum total time to spend retrying.
	// If Timeout is 0, there is no time limit.
	Timeout time.Duration
}

// Poll executes the given function `fn` until it returns no error, or until the
// configured number of tries, timeout, or context cancellation is reached.
//
// The function respects the provided context for cancellation. If `opts.Timeout`
// is non-zero, the timeout is applied in addition to the provided context.
func Poll(ctx context.Context, fn PollFunc, opts PollOptions) error {
    var lastErr error

    // If neither tries nor timeout are set, default to a single try (legacy behavior)
    if opts.Tries == 0 && opts.Timeout == 0 {
        opts.Tries = 1
    }

    // If a timeout is specified, derive a child context so we can cancel after timeout.
    if opts.Timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
        defer cancel()
    }

    const maxDelay = 30 * time.Second

    for i := 0; opts.Tries == 0 || i < opts.Tries; i++ {
        select {
        case <-ctx.Done():
            // prefer returning the context error
            return ctx.Err()
        default:
        }

        err := fn()
        if err == nil {
            return nil
        }
        lastErr = err

        if opts.Tries > 0 && i == opts.Tries-1 {
            break
        }

        var wait time.Duration
        if opts.Delay > 0 {
            wait = opts.Delay
        } else {
            // Binary exponential backoff with initial 2 seconds: 2^i * 2s
            wait = time.Duration(math.Pow(2, float64(i))) * 2 * time.Second
            if wait > maxDelay {
                wait = maxDelay
            }
        }

        // Sleep but wake early if context is done
        select {
        case <-time.After(wait):
            // continue to next iteration
        case <-ctx.Done():
            return ctx.Err()
        }
    }

    if lastErr != nil {
        if opts.Tries > 0 {
            return fmt.Errorf("after %d attempts, last error: %w", opts.Tries, lastErr)
        }
        return fmt.Errorf("last error: %w", lastErr)
    }

    return fmt.Errorf("polling failed without returning an error")
}
