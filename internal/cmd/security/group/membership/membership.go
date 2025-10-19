package membership

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/group/membership/add"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/group/membership/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "membership",
		Short:   "Manage security group memberships",
		Aliases: []string{"m"},
	}

	cmd.AddCommand(
		list.NewCmd(ctx),
		add.NewCmd(ctx),
	)
	return cmd
}
