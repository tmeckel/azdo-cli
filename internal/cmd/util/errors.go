package util

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/AlecAivazis/survey/v2/terminal"
)

// FlagErrorf returns a new ErrFlag that wraps an error produced by
// fmt.Errorf(format, args...).
func FlagErrorf(format string, args ...interface{}) error {
	return FlagErrorWrap(fmt.Errorf(format, args...))
}

// ErrFlag returns a new ErrFlag that wraps the specified error.
func FlagErrorWrap(err error) error { return &ErrFlag{err} }

// A *ErrFlag indicates an error processing command-line flags or other arguments.
// Such errors cause the application to display the usage message.
type ErrFlag struct {
	// Note: not struct{error}: only *ErrFlag should satisfy error.
	err error
}

func (fe *ErrFlag) Error() string {
	return fe.err.Error()
}

func (fe *ErrFlag) Unwrap() error {
	return fe.err
}

// SilentError is an error that triggers exit code 1 without any error messaging
var ErrSilent = errors.New("SilentError")

// CancelError signals user-initiated cancellation
var ErrCancel = errors.New("CancelError")

func IsUserCancellation(err error) bool {
	return errors.Is(err, ErrCancel) || errors.Is(err, terminal.InterruptErr)
}

func MutuallyExclusive(message string, conditions ...bool) error {
	numTrue := 0
	for _, ok := range conditions {
		if ok {
			numTrue++
		}
	}
	if numTrue > 1 {
		return FlagErrorf("%s", message)
	}
	return nil
}

type ErrNoResults struct {
	message string
}

func (e ErrNoResults) Error() string {
	return e.message
}

func NewNoResultsError(message string) ErrNoResults {
	return ErrNoResults{message: message}
}

type ErrExternalCommandExit struct {
	err *exec.ExitError
}

func (e ErrExternalCommandExit) Error() string {
	return e.err.Error()
}

func (e ErrExternalCommandExit) ExitCode() int {
	return e.err.ExitCode()
}

func NewExternalCommandExitError(err *exec.ExitError) ErrExternalCommandExit {
	return ErrExternalCommandExit{
		err: err,
	}
}
