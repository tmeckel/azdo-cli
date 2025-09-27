package util

import (
	"errors"
	"fmt"
	"net/http"
	"os/exec"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
)

// FlagErrorf returns a new FlagError that wraps an error produced by
// fmt.Errorf(format, args...).
func FlagErrorf(format string, args ...any) error {
	return FlagErrorWrap(fmt.Errorf(format, args...))
}

// FlagError returns a new FlagError that wraps the specified error.
func FlagErrorWrap(err error) error { return &FlagError{err} }

// A *FlagError indicates an error processing command-line flags or other arguments.
// Such errors cause the application to display the usage message.
type FlagError struct {
	// Note: not struct{error}: only *FlagError should satisfy error.
	err error
}

func (fe *FlagError) Error() string {
	return fe.err.Error()
}

func (fe *FlagError) Unwrap() error {
	return fe.err
}

// ErrSilent is an error that triggers exit code 1 without any error messaging
var ErrSilent = errors.New("SilentError")

// ErrCancel signals user-initiated cancellation
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

type NoResultsError struct {
	message string
}

func (e NoResultsError) Error() string {
	return e.message
}

// Is allows errors.Is to match any NoResultsError regardless of message content.
func (e NoResultsError) Is(target error) bool {
	if _, ok := target.(NoResultsError); ok {
		return true
	}
	if _, ok := target.(*NoResultsError); ok {
		return true
	}
	return false
}

func NewNoResultsError(message string) NoResultsError {
	return NoResultsError{message: message}
}

type ExternalCommandExitError struct {
	err *exec.ExitError
}

func (e ExternalCommandExitError) Error() string {
	return e.err.Error()
}

func (e ExternalCommandExitError) ExitCode() int {
	return e.err.ExitCode()
}

func NewExternalCommandExitError(err *exec.ExitError) ExternalCommandExitError {
	return ExternalCommandExitError{
		err: err,
	}
}

// ErrNotImplemented is an error that indicates a feature is not implemented
var ErrNotImplemented = errors.New("NotImplementedError")

// IsNotFoundError checks if the given error is an Azure DevOps API "Not Found" (HTTP 404) error.
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// Traverse the error chain to find a WrappedError
	for {
		var wrappedErr *azuredevops.WrappedError
		if errors.As(err, &wrappedErr) {
			if wrappedErr.StatusCode != nil && *wrappedErr.StatusCode == http.StatusNotFound {
				return true
			}
			// If WrappedError has an InnerError, continue traversing
			if wrappedErr.InnerError != nil {
				err = wrappedErr.InnerError
				continue
			}
		}

		// If the error is not a WrappedError or has no InnerError, stop traversing
		break
	}

	return false
}
