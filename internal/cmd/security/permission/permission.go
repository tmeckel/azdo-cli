package permission

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/permission/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/permission/namespace"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/permission/show"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/permission/update"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "permission",
		Short: "Manage Azure DevOps security permissions.",
		Aliases: []string{
			"p",
			"perm",
			"permissions",
		},
	}

	cmd.AddCommand(list.NewCmd(ctx))
	cmd.AddCommand(namespace.NewCmd(ctx))
	cmd.AddCommand(show.NewCmd(ctx))
	cmd.AddCommand(update.NewCmd(ctx))

	return cmd
}
