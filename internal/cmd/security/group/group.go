package group

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/group/create"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/group/delete"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/group/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/group/membership"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/group/show"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/group/update"
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
	cmd.AddCommand(delete.NewCmd(ctx))
	cmd.AddCommand(list.NewCmd(ctx))
	cmd.AddCommand(membership.NewCmd(ctx))
	cmd.AddCommand(show.NewCmd(ctx))
	cmd.AddCommand(update.NewCmd(ctx))

	return cmd
}
