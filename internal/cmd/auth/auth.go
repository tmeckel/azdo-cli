package auth

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/auth/gitcredential"
	"github.com/tmeckel/azdo-cli/internal/cmd/auth/login"
	"github.com/tmeckel/azdo-cli/internal/cmd/auth/logout"
	"github.com/tmeckel/azdo-cli/internal/cmd/auth/setupgit"
	"github.com/tmeckel/azdo-cli/internal/cmd/auth/status"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmdAuth(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "auth <command>",
		Short:   "Authenticate azdo and git with Azure DevOps",
		GroupID: "core",
	}

	util.DisableAuthCheck(cmd)

	cmd.AddCommand(gitcredential.NewCmdGitCredential(ctx))
	cmd.AddCommand(login.NewCmdLogin(ctx))
	cmd.AddCommand(logout.NewCmdLogout(ctx))
	cmd.AddCommand(status.NewCmdStatus(ctx))
	cmd.AddCommand(setupgit.NewCmdSetupGit(ctx))

	return cmd
}
