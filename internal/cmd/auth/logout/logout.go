package logout

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmdLogout(ctx util.CmdContext) *cobra.Command {
	return nil
}

// Logout must
// 1. Check if the organization set as default, if yes: clear default
// 2. Remove global credential helper (azdo auth setup-git)
// 3. Remove the organization from the config
