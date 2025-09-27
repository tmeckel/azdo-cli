package graph

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/graph/user"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmdGraph(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph <command>",
		Short: "Manage Azure DevOps Graph resources (users, groups)",
		Long: heredoc.Doc(`
            Work with Azure DevOps Graph resources such as users and groups.
            The Graph API allows you to manage users, groups, and their memberships.
        `),
		GroupID: "core",
	}

	cmd.AddCommand(user.NewCmd(ctx))
	return cmd
}
