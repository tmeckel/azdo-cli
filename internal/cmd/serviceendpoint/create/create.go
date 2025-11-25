package create

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/create/azurerm"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create service connections",
	}

	cmd.AddCommand(azurerm.NewCmd(ctx))

	return cmd
}
