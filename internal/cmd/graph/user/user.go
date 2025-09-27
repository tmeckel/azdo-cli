package user

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/graph/user/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

// NewCmd returns the parent "graph user" command that groups user-related subcommands.
func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user <command>",
		Short: "Manage users in Azure DevOps",
		Long: heredoc.Doc(`
            Commands to query and manage users in Azure DevOps using the Graph API.
        `),
	}

	cmd.AddCommand(list.NewCmd(ctx))
	return cmd
}
