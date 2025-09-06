package util

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/azdo"
)

func EnableRepoOverride(ctx CmdContext, cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		cmd.PersistentFlags().StringP("repo", "R", "", "Select another repository using the `[ORG/]PROJECT/REPO` format")
		cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Parsed() {
				return nil
			}

			if err := executeParentHooks(cmd, args); err != nil {
				return err
			}

			repoOverride, err := cmd.Flags().GetString("repo")
			if err != nil {
				return err
			}
			ctx.RepoContext().WithRepo(overrideRepo(ctx, repoOverride))
			return nil
		}
	}
}

func executeParentHooks(cmd *cobra.Command, args []string) error {
	for cmd.HasParent() {
		cmd = cmd.Parent()
		if cmd.PersistentPreRunE != nil {
			return cmd.PersistentPreRunE(cmd, args)
		}
	}
	return nil
}

func overrideRepo(_ CmdContext, override string) func() (azdo.Repository, error) {
	if override == "" {
		override = os.Getenv("AZDO_REPO")
	}
	if override != "" {
		return func() (azdo.Repository, error) {
			return azdo.RepositoryFromName(override)
		}
	}
	return nil
}
