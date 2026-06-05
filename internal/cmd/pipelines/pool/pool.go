package pool

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/pool/show"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool",
		Short: "Manage agent pools",
		Long: heredoc.Doc(`
			Manage Azure DevOps agent pools. Agent pools are logical groupings
			of agents that target build, release, and other pipeline jobs.
		`),
		Example: heredoc.Doc(`
			# Show a pool by ID
			azdo pipelines pool show 42

			# Show a pool by name
			azdo pipelines pool show 'Default'

			# Show a pool in a specific organization
			azdo pipelines pool show 'myorg/Default'
		`),
		Aliases: []string{"pools"},
	}

	cmd.AddCommand(show.NewCmd(ctx))
	return cmd
}
