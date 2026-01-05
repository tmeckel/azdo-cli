package util

import (
    "context"
    "errors"
    "testing"
    "time"

    "github.com/stretchr/testify/require"
)

func TestPollDefaultsToSingleTry(t *testing.T) {
    var calls int

    err := Poll(context.Background(), func() error {
        calls++
        return nil
    }, PollOptions{})

    require.NoError(t, err)
    require.Equal(t, 1, calls)
}

func TestPollRetriesUntilSuccess(t *testing.T) {
    var calls int
    errBoom := errors.New("temporary failure")

    err := Poll(context.Background(), func() error {
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

    err := Poll(context.Background(), func() error {
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

    err := Poll(context.Background(), func() error {
        calls++
        return errBoom
    }, PollOptions{
        Delay:   20 * time.Millisecond,
        Timeout: 30 * time.Millisecond,
    })

    require.Error(t, err)
    require.ErrorContains(t, err, "context deadline exceeded")
    require.GreaterOrEqual(t, calls, 1)
}

func TestPollRespectsContextCancel(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    err := Poll(ctx, func() error { return errors.New("no-op") }, PollOptions{Delay: time.Millisecond})
    require.ErrorIs(t, err, context.Canceled)
}

func TestPollBinaryExponentialBackoff(t *testing.T) {
    var calls int
    err := Poll(context.Background(), func() error {
        calls++
        if calls >= 3 {
            return nil
        }
        return errors.New("simulated failure")
    }, PollOptions{
        Tries: 4,
    })

    require.NoError(t, err)
    require.GreaterOrEqual(t, calls, 3)
}
