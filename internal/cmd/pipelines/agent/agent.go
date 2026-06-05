package agent

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/agent/show"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage Azure DevOps pipeline agents",
		Example: heredoc.Doc(`
			# Show details of an agent
			azdo pipelines agent show 1/42
		`),
		Aliases: []string{"agents", "a"},
	}

	cmd.AddCommand(show.NewCmd(ctx))
	return cmd
}
