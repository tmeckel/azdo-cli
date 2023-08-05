package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/build"
	"github.com/tmeckel/azdo-cli/internal/cmd/root"
	cmdutil "github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/util"
	"go.uber.org/zap"
)

type exitCode int

const (
	exitOK     exitCode = 0
	exitError  exitCode = 1
	exitCancel exitCode = 2
	exitAuth   exitCode = 4
)

func init() {
	// Initialize ZAP logger package
	var logger *zap.Logger
	var err error
	if debug, _ := util.IsDebugEnabled(); debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	zap.ReplaceGlobals(zap.Must(logger, err))
}

func main() {
	code := mainRun()
	os.Exit(int(code))
}

func mainRun() exitCode {
	buildDate := build.Date
	buildVersion := build.Version

	zap.L().Sugar().Debugf("Version %s, Date %+v", buildVersion, buildDate)
	cmdCtx, err := cmdutil.NewCmdContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create command context: %s", err)
		return exitError
	}

	iostrms, err := cmdCtx.IOStreams()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get IOStreams: %s", err)
		return exitError
	}

	rootCmd, err := root.NewCmdRoot(cmdCtx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create root command: %s", err)
		return exitError
	}

	if cmd, err := rootCmd.ExecuteC(); err != nil {
		var pagerPipeError *iostreams.ErrClosedPagerPipe
		var noResultsError cmdutil.ErrNoResults
		var extError *cmdutil.ErrExternalCommandExit
		var authError *root.AuthError
		stderr := iostrms.ErrOut

		if err == cmdutil.ErrSilent {
			return exitError
		} else if cmdutil.IsUserCancellation(err) {
			if errors.Is(err, terminal.InterruptErr) {
				// ensure the next shell prompt will start on its own line
				fmt.Fprint(stderr, "\n")
			}
			return exitCancel
		} else if errors.As(err, &authError) {
			return exitAuth
		} else if errors.As(err, &pagerPipeError) {
			// ignore the error raised when piping to a closed pager
			return exitOK
		} else if errors.As(err, &noResultsError) {
			if iostrms.IsStdoutTTY() {
				fmt.Fprintln(stderr, noResultsError.Error())
			}
			// no results is not a command failure
			return exitOK
		} else if errors.As(err, &extError) {
			// pass on exit codes from extensions and shell aliases
			return exitCode(extError.ExitCode())
		}

		printError(stderr, err, cmd)

		if strings.Contains(err.Error(), "Incorrect function") {
			fmt.Fprintln(stderr, "You appear to be running in MinTTY without pseudo terminal support.")
			fmt.Fprintln(stderr, "To learn about workarounds for this error, run:  gh help mintty")
			return exitError
		}

		return exitError
	}

	return exitOK
}

func printError(out io.Writer, err error, cmd *cobra.Command) {
	var dnsError *net.DNSError
	if errors.As(err, &dnsError) {
		debug, _ := util.IsDebugEnabled()
		fmt.Fprintf(out, "error connecting to %s\n", dnsError.Name)
		if debug {
			fmt.Fprintln(out, dnsError)
		}
		fmt.Fprintln(out, "check your internet connection or https://status.dev.azure.com")
		return
	}

	fmt.Fprintln(out, err)

	var flagError *cmdutil.ErrFlag
	if errors.As(err, &flagError) || strings.HasPrefix(err.Error(), "unknown command ") {
		if !strings.HasSuffix(err.Error(), "\n") {
			fmt.Fprintln(out)
		}
		fmt.Fprintln(out, cmd.UsageString())
	}
}
