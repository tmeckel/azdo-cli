package variablegroups

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroups/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "variable-groups",
		Short: "Manage Azure DevOps variable groups",
		Aliases: []string{
			"variable-groups",
			"variablegroups",
			"vg",
		},
	}

	cmd.AddCommand(list.NewCmd(ctx))
	return cmd
}
