package root

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/google/shlex"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/run"
	"github.com/tmeckel/azdo-cli/internal/text"
)

func NewCmdShellAlias(ctx util.CmdContext, aliasName, aliasValue string) (cmd *cobra.Command, err error) {
	iostrms, err := ctx.IOStreams()
	if err != nil {
		return cmd, err
	}
	cmd = &cobra.Command{
		Use:   aliasName,
		Short: fmt.Sprintf("Shell alias for %q", text.Truncate(80, aliasValue)),
		RunE: func(c *cobra.Command, args []string) error {
			expandedArgs, err := expandShellAlias(aliasValue, args, nil)
			if err != nil {
				return err
			}
			externalCmd := exec.Command(expandedArgs[0], expandedArgs[1:]...)
			externalCmd.Stderr = iostrms.ErrOut
			externalCmd.Stdout = iostrms.Out
			externalCmd.Stdin = iostrms.In
			preparedCmd := run.PrepareCmd(externalCmd)
			if err = preparedCmd.Run(); err != nil {
				var execError *exec.ExitError
				if errors.As(err, &execError) {
					return util.NewExternalCommandExitError(execError)
				}
				return fmt.Errorf("failed to run external command: %w", err)
			}
			return nil
		},
		GroupID: "alias",
		Annotations: map[string]string{
			"skipAuthCheck": "true",
		},
		DisableFlagParsing: true,
	}
	return cmd, err
}

func NewCmdAlias(ctx util.CmdContext, aliasName, aliasValue string) (cmd *cobra.Command, err error) {
	cmd = &cobra.Command{
		Use:   aliasName,
		Short: fmt.Sprintf("Alias for %q", text.Truncate(80, aliasValue)),
		RunE: func(c *cobra.Command, args []string) error {
			expandedArgs, err := expandAlias(aliasValue, args)
			if err != nil {
				return err
			}
			root := c.Root()
			root.SetArgs(expandedArgs)
			return root.Execute()
		},
		GroupID: "alias",
		Annotations: map[string]string{
			"skipAuthCheck": "true",
		},
		DisableFlagParsing: true,
	}
	return cmd, err
}

// ExpandAlias processes argv to see if it should be rewritten according to a user's aliases.
func expandAlias(expansion string, args []string) ([]string, error) {
	extraArgs := []string{}
	for i, a := range args {
		if !strings.Contains(expansion, "$") {
			extraArgs = append(extraArgs, a)
		} else {
			expansion = strings.ReplaceAll(expansion, fmt.Sprintf("$%d", i+1), a)
		}
	}

	lingeringRE := regexp.MustCompile(`\$\d`)
	if lingeringRE.MatchString(expansion) {
		return nil, fmt.Errorf("not enough arguments for alias: %s", expansion)
	}

	newArgs, err := shlex.Split(expansion)
	if err != nil {
		return nil, err
	}

	return append(newArgs, extraArgs...), nil
}

// ExpandShellAlias processes argv to see if it should be rewritten according to a user's aliases.
func expandShellAlias(expansion string, args []string, findShFunc func() (string, error)) ([]string, error) {
	if findShFunc == nil {
		findShFunc = findSh
	}

	shPath, shErr := findShFunc()
	if shErr != nil {
		return nil, shErr
	}

	expanded := []string{shPath, "-c", expansion[1:]}

	if len(args) > 0 {
		expanded = append(expanded, "--")
		expanded = append(expanded, args...)
	}

	return expanded, nil
}

func findSh() (string, error) {
	shPath, err := exec.LookPath("sh")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			if runtime.GOOS == "windows" {
				return "", errors.New("unable to locate sh to execute the shell alias with. The sh.exe interpreter is typically distributed with Git for Windows")
			}
			return "", errors.New("unable to locate sh to execute shell alias with")
		}
		return "", err
	}
	return shPath, nil
}
