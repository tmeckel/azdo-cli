package security

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/group"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/permission"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "security <command> [flags]",
		Short: "Work with Azure DevOps security.",
		Long:  "Work with Azure DevOps security features.",
		Aliases: []string{
			"s",
			"sec",
		},
		GroupID: "security",
	}

	cmd.AddCommand(group.NewCmd(ctx))
	cmd.AddCommand(permission.NewCmd(ctx))

	return cmd
}
