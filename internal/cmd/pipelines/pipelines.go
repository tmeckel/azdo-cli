package pipelines

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/agent"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/build"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/delete"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/pool"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/queue"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/run"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/runs"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/show"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pipelines",
		Short:   "Manage Azure DevOps pipelines",
		Aliases: []string{"p"},
		Example: heredoc.Doc(`
			# Delete a pipeline definition
			azdo pipelines delete Fabrikam/42 --yes
		`),
	}

	cmd.AddCommand(build.NewCmd(ctx))
	cmd.AddCommand(delete.NewCmd(ctx))
	cmd.AddCommand(list.NewCmd(ctx))
	cmd.AddCommand(run.NewCmd(ctx))
	cmd.AddCommand(runs.NewCmd(ctx))
	cmd.AddCommand(show.NewCmd(ctx))
	cmd.AddCommand(variablegroup.NewCmd(ctx))
	cmd.AddCommand(agent.NewCmd(ctx))
	cmd.AddCommand(pool.NewCmd(ctx))
	cmd.AddCommand(queue.NewCmd(ctx))
	return cmd
}
