package runs

import (
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/runs/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/runs/show"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "Manage pipeline runs",
		Long:  "Manage pipeline runs in an Azure DevOps project.",
	}

	cmd.AddCommand(list.NewCmd(ctx))
	cmd.AddCommand(show.NewCmd(ctx))
	return cmd
}
