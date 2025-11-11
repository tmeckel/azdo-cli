package test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPollDefaultsToSingleTry(t *testing.T) {
	var calls int

	err := Poll(func() error {
		calls++
		return nil
	}, PollOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, calls)
}

func TestPollRetriesUntilSuccess(t *testing.T) {
	var calls int
	errBoom := errors.New("temporary failure")

	err := Poll(func() error {
		calls++
		if calls < 3 {
			return errBoom
		}

		return nil
	}, PollOptions{
		Tries: 5,
		Delay: time.Millisecond,
	})

	require.NoError(t, err)
	require.Equal(t, 3, calls)
}

func TestPollReturnsAfterMaxAttempts(t *testing.T) {
	var calls int
	errBoom := errors.New("permanent failure")

	err := Poll(func() error {
		calls++
		return errBoom
	}, PollOptions{
		Tries: 3,
		Delay: time.Millisecond,
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "after 3 attempts")
	require.ErrorIs(t, err, errBoom)
	require.Equal(t, 3, calls)
}

func TestPollTimeout(t *testing.T) {
	var calls int
	errBoom := errors.New("timeout failure")

	err := Poll(func() error {
		calls++
		return errBoom
	}, PollOptions{
		Delay:   20 * time.Millisecond,
		Timeout: 30 * time.Millisecond,
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "timed out after")
	require.ErrorContains(t, err, errBoom.Error())
	require.GreaterOrEqual(t, calls, 2)
}

func TestPollBinaryExponentialBackoff(t *testing.T) {
	// Expected waits: 2s, 4s, 8s (formula: 2^i * 2s)
	expectedDurations := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}
	var lastCall time.Time
	var callIndex int

	err := Poll(func() error {
		now := time.Now()
		if callIndex > 0 {
			elapsed := now.Sub(lastCall)
			expected := expectedDurations[callIndex-1]

			// Allow ±10% margin for scheduler jitter
			min := expected - expected/10
			max := expected + expected/10
			if elapsed < min || elapsed > max {
				t.Fatalf("Call %d: expected ~%v wait, got %v", callIndex, expected, elapsed)
			}
		}
		if callIndex >= len(expectedDurations) {
			return nil // succeed after verifying waits
		}
		lastCall = now
		callIndex++
		return errors.New("simulated failure")
	}, PollOptions{
		Tries: len(expectedDurations) + 1, // enough tries to verify all intervals
	}) // Delay left at zero

	require.NoError(t, err)
}
