package test

import (
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
// configured number of tries or timeout is reached.
func Poll(fn PollFunc, opts PollOptions) error {
	var lastErr error

	if opts.Tries == 0 && opts.Timeout == 0 {
		opts.Tries = 1
	}

	startTime := time.Now()
	for i := 0; opts.Tries == 0 || i < opts.Tries; i++ {
		if opts.Timeout > 0 && time.Since(startTime) > opts.Timeout {
			if lastErr != nil {
				return fmt.Errorf("timed out after %v, last error: %w", opts.Timeout, lastErr)
			}
			return fmt.Errorf("timed out after %v", opts.Timeout)
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
			// Binary exponential backoff with initial 2 seconds
			wait = time.Duration(math.Pow(2, float64(i))) * 2 * time.Second
		}
		time.Sleep(wait)
	}

	if lastErr != nil {
		if opts.Tries > 0 {
			return fmt.Errorf("after %d attempts, last error: %w", opts.Tries, lastErr)
		}
		return fmt.Errorf("last error: %w", lastErr)
	}

	return fmt.Errorf("polling failed without returning an error")
}
