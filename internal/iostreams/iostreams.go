package iostreams

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"

	"github.com/cli/safeexec"
	"github.com/google/shlex"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/pterm/pterm"
	azdoTerm "github.com/tmeckel/azdo-cli/internal/term"
)

const DefaultWidth = 80

// ErrClosedPagerPipe is the error returned when writing to a pager that has been closed.
type ErrClosedPagerPipe struct {
	error
}

type fileWriter interface {
	io.Writer
	Fd() uintptr
}

type fileReader interface {
	io.ReadCloser
	Fd() uintptr
}

type term interface {
	IsTerminalOutput() bool
	Size() (int, int, error)
}

type IOStreams struct {
	term term

	In     fileReader
	Out    fileWriter
	ErrOut io.Writer

	terminalTheme string

	alternateScreenBufferEnabled bool
	alternateScreenBufferActive  bool
	alternateScreenBufferMu      sync.Mutex

	stdinTTYOverride  bool
	stdinIsTTY        bool
	stdoutTTYOverride bool
	stdoutIsTTY       bool
	stderrTTYOverride bool
	stderrIsTTY       bool

	pagerCommand string
	pagerProcess *os.Process

	neverPrompt bool

	TempFileOverride *os.File
}

func (ios *IOStreams) SetStdinTTY(isTTY bool) {
	ios.stdinTTYOverride = true
	ios.stdinIsTTY = isTTY
}

func (ios *IOStreams) IsStdinTTY() bool {
	if ios.stdinTTYOverride {
		return ios.stdinIsTTY
	}
	if stdin, ok := ios.In.(*os.File); ok {
		return isTerminal(stdin)
	}
	return false
}

func (ios *IOStreams) SetStdoutTTY(isTTY bool) {
	ios.stdoutTTYOverride = true
	ios.stdoutIsTTY = isTTY
}

func (ios *IOStreams) IsStdoutTTY() bool {
	if ios.stdoutTTYOverride {
		return ios.stdoutIsTTY
	}
	// support AZDO_FORCE_TTY
	if ios.term.IsTerminalOutput() {
		return true
	}
	stdout, ok := ios.Out.(*os.File)
	return ok && isCygwinTerminal(stdout.Fd())
}

func (ios *IOStreams) SetStderrTTY(isTTY bool) {
	ios.stderrTTYOverride = true
	ios.stderrIsTTY = isTTY
}

func (ios *IOStreams) IsStderrTTY() bool {
	if ios.stderrTTYOverride {
		return ios.stderrIsTTY
	}
	if stderr, ok := ios.ErrOut.(*os.File); ok {
		return isTerminal(stderr)
	}
	return false
}

func (ios *IOStreams) SetPager(cmd string) {
	ios.pagerCommand = cmd
}

func (ios *IOStreams) GetPager() string {
	return ios.pagerCommand
}

func (ios *IOStreams) StartPager() error {
	if ios.pagerCommand == "" || ios.pagerCommand == "cat" || !ios.IsStdoutTTY() {
		return nil
	}

	pagerArgs, err := shlex.Split(ios.pagerCommand)
	if err != nil {
		return err
	}

	pagerEnv := os.Environ()
	for i := len(pagerEnv) - 1; i >= 0; i-- {
		if strings.HasPrefix(pagerEnv[i], "PAGER=") {
			pagerEnv = append(pagerEnv[0:i], pagerEnv[i+1:]...)
		}
	}
	if _, ok := os.LookupEnv("LESS"); !ok {
		pagerEnv = append(pagerEnv, "LESS=FRX")
	}
	if _, ok := os.LookupEnv("LV"); !ok {
		pagerEnv = append(pagerEnv, "LV=-c")
	}

	pagerExe, err := safeexec.LookPath(pagerArgs[0])
	if err != nil {
		return err
	}
	pagerCmd := exec.Command(pagerExe, pagerArgs[1:]...)
	pagerCmd.Env = pagerEnv
	pagerCmd.Stdout = ios.Out
	pagerCmd.Stderr = ios.ErrOut
	pagedOut, err := pagerCmd.StdinPipe()
	if err != nil {
		return err
	}
	ios.Out = &fdWriteCloser{
		fd:          ios.Out.Fd(),
		WriteCloser: &pagerWriter{pagedOut},
	}
	err = pagerCmd.Start()
	if err != nil {
		return err
	}
	ios.pagerProcess = pagerCmd.Process
	return nil
}

func (ios *IOStreams) StopPager() {
	if ios.pagerProcess == nil {
		return
	}

	// if a pager was started, we're guaranteed to have a WriteCloser
	_ = ios.Out.(io.WriteCloser).Close()
	_, _ = ios.pagerProcess.Wait()
	ios.pagerProcess = nil
}

func (ios *IOStreams) CanPrompt() bool {
	if ios.neverPrompt {
		return false
	}

	return ios.IsStdinTTY() && ios.IsStdoutTTY()
}

func (ios *IOStreams) GetNeverPrompt() bool {
	return ios.neverPrompt
}

func (ios *IOStreams) SetNeverPrompt(v bool) {
	ios.neverPrompt = v
}

func (ios *IOStreams) RunWithProgress(label string, run func(s *pterm.SpinnerPrinter) error) error {
	s, err := pterm.DefaultSpinner.Start(label)
	if err != nil {
		return err
	}
	err = run(s)
	if err != nil {
		s.Fail()
	}
	s.Success()
	return err
}

func (ios *IOStreams) StartAlternateScreenBuffer() {
	if ios.alternateScreenBufferEnabled {
		ios.alternateScreenBufferMu.Lock()
		defer ios.alternateScreenBufferMu.Unlock()

		if _, err := fmt.Fprint(ios.Out, "\x1b[?1049h"); err == nil {
			ios.alternateScreenBufferActive = true

			ch := make(chan os.Signal, 1)
			signal.Notify(ch, os.Interrupt)

			go func() {
				<-ch
				ios.StopAlternateScreenBuffer()

				os.Exit(1)
			}()
		}
	}
}

func (ios *IOStreams) StopAlternateScreenBuffer() {
	ios.alternateScreenBufferMu.Lock()
	defer ios.alternateScreenBufferMu.Unlock()

	if ios.alternateScreenBufferActive {
		fmt.Fprint(ios.Out, "\x1b[?1049l")
		ios.alternateScreenBufferActive = false
	}
}

func (ios *IOStreams) SetAlternateScreenBufferEnabled(enabled bool) {
	ios.alternateScreenBufferEnabled = enabled
}

func (ios *IOStreams) RefreshScreen() {
	if ios.IsStdoutTTY() {
		// Move cursor to 0,0
		fmt.Fprint(ios.Out, "\x1b[0;0H")
		// Clear from cursor to bottom of screen
		fmt.Fprint(ios.Out, "\x1b[J")
	}
}

// TerminalWidth returns the width of the terminal that controls the process
func (ios *IOStreams) TerminalWidth() int {
	w, _, err := ios.term.Size()
	if err == nil && w > 0 {
		return w
	}
	return DefaultWidth
}

func (ios *IOStreams) ReadUserFile(fn string) ([]byte, error) {
	var r io.ReadCloser
	if fn == "-" {
		r = ios.In
	} else {
		var err error
		r, err = os.Open(fn)
		if err != nil {
			return nil, err
		}
	}
	defer r.Close()
	return io.ReadAll(r)
}

func (ios *IOStreams) TempFile(dir, pattern string) (*os.File, error) {
	if ios.TempFileOverride != nil {
		return ios.TempFileOverride, nil
	}
	return os.CreateTemp(dir, pattern)
}

func System() *IOStreams {
	terminal := azdoTerm.FromEnv()

	var stdout fileWriter = os.Stdout
	// On Windows with no virtual terminal processing support, translate ANSI escape
	// sequences to console syscalls
	if colorableStdout := colorable.NewColorable(os.Stdout); colorableStdout != os.Stdout {
		// ensure that the file descriptor of the original stdout is preserved
		stdout = &fdWriter{
			fd:     os.Stdout.Fd(),
			Writer: colorableStdout,
		}
	}

	io := &IOStreams{
		In:           os.Stdin,
		Out:          stdout,
		ErrOut:       colorable.NewColorable(os.Stderr),
		pagerCommand: os.Getenv("PAGER"),
		term:         &terminal,
	}

	if io.IsStdoutTTY() && hasAlternateScreenBuffer(terminal.IsTrueColorSupported()) {
		io.alternateScreenBufferEnabled = true
	}

	return io
}

type fakeTerm struct{}

func (t fakeTerm) IsTerminalOutput() bool {
	return false
}

func (t fakeTerm) IsColorEnabled() bool {
	return false
}

func (t fakeTerm) Is256ColorSupported() bool {
	return false
}

func (t fakeTerm) IsTrueColorSupported() bool {
	return false
}

func (t fakeTerm) Theme() string {
	return ""
}

func (t fakeTerm) Size() (int, int, error) {
	return 80, -1, nil
}

func Test() (*IOStreams, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	io := &IOStreams{
		In: &fdReader{
			fd:         0,
			ReadCloser: io.NopCloser(in),
		},
		Out:    &fdWriter{fd: 1, Writer: out},
		ErrOut: errOut,
		term:   &fakeTerm{},
	}
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)
	return io, in, out, errOut
}

func isTerminal(f *os.File) bool {
	return azdoTerm.IsTerminal(f) || isCygwinTerminal(f.Fd())
}

func isCygwinTerminal(fd uintptr) bool {
	return isatty.IsCygwinTerminal(fd)
}

// pagerWriter implements a WriteCloser that wraps all EPIPE errors in an ErrClosedPagerPipe type.
type pagerWriter struct {
	io.WriteCloser
}

func (w *pagerWriter) Write(d []byte) (int, error) {
	n, err := w.WriteCloser.Write(d)
	if err != nil && (errors.Is(err, io.ErrClosedPipe) || isEpipeError(err)) {
		return n, &ErrClosedPagerPipe{err}
	}
	return n, err
}

// fdWriter represents a wrapped stdout Writer that preserves the original file descriptor
type fdWriter struct {
	io.Writer
	fd uintptr
}

func (w *fdWriter) Fd() uintptr {
	return w.fd
}

// fdWriteCloser represents a wrapped stdout Writer that preserves the original file descriptor
type fdWriteCloser struct {
	io.WriteCloser
	fd uintptr
}

func (w *fdWriteCloser) Fd() uintptr {
	return w.fd
}

// fdWriter represents a wrapped stdin ReadCloser that preserves the original file descriptor
type fdReader struct {
	io.ReadCloser
	fd uintptr
}

func (r *fdReader) Fd() uintptr {
	return r.fd
}
