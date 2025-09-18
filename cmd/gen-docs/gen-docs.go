package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/pflag"
	"github.com/tmeckel/azdo-cli/cmd"
	"github.com/tmeckel/azdo-cli/internal/cmd/root"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/docs"
	"go.uber.org/zap"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	buildDate := cmd.Date
	buildVersion := cmd.Version

	zap.L().Sugar().Debugf("Version %s, Date %+v", buildVersion, buildDate)

	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	manPage := flags.BoolP("man-page", "", false, "Generate manual pages")
	website := flags.BoolP("website", "", false, "Generate website pages")
	dir := flags.StringP("doc-path", "", "", "Path directory where you want generate doc files")
	help := flags.BoolP("help", "h", false, "Help about any command")

	if err := flags.Parse(args); err != nil {
		return err
	}

	if *help {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n\n%s", filepath.Base(args[0]), flags.FlagUsages())
		return nil
	}

	if *dir == "" {
		return fmt.Errorf("error: --doc-path not set")
	}

	cmdCtx, err := util.NewCmdContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create command context: %s", err)
		return err
	}

	rootCmd, _ := root.NewCmdRoot(cmdCtx, "", "")
	rootCmd.InitDefaultHelpCmd()

	if err := os.MkdirAll(*dir, 0o755); err != nil { //nolint:gosec
		return err
	}

	if *website {
		if err := docs.GenMarkdownTreeCustom(rootCmd, *dir, filePrepender, linkHandler); err != nil {
			return err
		}
	}

	if *manPage {
		if err := docs.GenManTree(rootCmd, *dir); err != nil {
			return err
		}
	}

	return nil
}

func filePrepender(filename string) string {
	return ""
}

func linkHandler(name string) string {
	return fmt.Sprintf("./%s", name)
}
