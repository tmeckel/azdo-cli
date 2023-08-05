package git

import (
	"errors"
	"fmt"
)

// ErrNotOnAnyBranch indicates that the user is in detached HEAD state.
var ErrNotOnAnyBranch = errors.New("git: not on any branch")

type NotInstalledError struct {
	message string
	err     error
}

func (e *NotInstalledError) Error() string {
	return e.message
}

func (e *NotInstalledError) Unwrap() error {
	return e.err
}

type Error struct {
	ExitCode int
	Stderr   string
	err      error
}

func (ge *Error) Error() string {
	if ge.Stderr == "" {
		return fmt.Sprintf("failed to run git: %v", ge.err)
	}
	return fmt.Sprintf("failed to run git: %s", ge.Stderr)
}

func (ge *Error) Unwrap() error {
	return ge.err
}
