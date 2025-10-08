package group

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/group/create"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/group/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Manage security groups",
		Long:  "Manage security groups in Azure DevOps.",
		Aliases: []string{
			"g",
			"grp",
		},
	}

	cmd.AddCommand(create.NewCmd(ctx))
	cmd.AddCommand(list.NewCmd(ctx))

	return cmd
}
