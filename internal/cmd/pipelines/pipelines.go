package pipelines

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pipelines",
		Short:   "Manage Azure DevOps pipelines",
		Aliases: []string{"p"},
	}

	cmd.AddCommand(variablegroup.NewCmd(ctx))
	return cmd
}
