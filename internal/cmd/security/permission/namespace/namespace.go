package namespace

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/permission/namespace/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/permission/namespace/show"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "namespace",
		Short: "Inspect security permission namespaces.",
		Aliases: []string{
			"n",
			"ns",
		},
	}

	cmd.AddCommand(list.NewCmd(ctx))
	cmd.AddCommand(show.NewCmd(ctx))

	return cmd
}
