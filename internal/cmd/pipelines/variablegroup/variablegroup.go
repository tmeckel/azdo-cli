package variablegroup

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup/create"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup/variable"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "variable-group",
		Short: "Manage Azure DevOps variable groups",
		Aliases: []string{
			"variablegroup",
			"variable-groups",
			"variablegroups",
			"vg",
		},
	}

	cmd.AddCommand(list.NewCmd(ctx))
	cmd.AddCommand(create.NewCmd(ctx))
	cmd.AddCommand(variable.NewCmd(ctx))
	return cmd
}
