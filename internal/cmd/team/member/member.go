package member

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/team/member/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "member <command>",
		Short: "Manage members of a team.",
	}

	cmd.AddCommand(list.NewCmd(ctx))

	return cmd
}
