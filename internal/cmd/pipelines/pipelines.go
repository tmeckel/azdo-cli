package pipelines

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/agent"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/pool"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pipelines",
		Short:   "Manage Azure DevOps pipelines",
		Aliases: []string{"p"},
	}

	cmd.AddCommand(list.NewCmd(ctx))
	cmd.AddCommand(variablegroup.NewCmd(ctx))
	cmd.AddCommand(agent.NewCmd(ctx))
	cmd.AddCommand(pool.NewCmd(ctx))
	return cmd
}
