package queue

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/queue/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/queue/show"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Manage Azure DevOps agent queues",
		Long: heredoc.Doc(`
			Manage Azure DevOps agent queues. Queues are project-scoped
			and connect a project to an agent pool.
		`),
		Example: heredoc.Doc(`
			# List queues in a project
			azdo pipelines queue list Fabrikam
		`),
	}

	cmd.AddCommand(list.NewCmd(ctx))
	cmd.AddCommand(show.NewCmd(ctx))
	return cmd
}
