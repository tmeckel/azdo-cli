package config

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/config/get"
	"github.com/tmeckel/azdo-cli/internal/cmd/config/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/config/set"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/config"
)

func NewCmdConfig(ctx util.CmdContext) *cobra.Command {
	longDoc := strings.Builder{}
	longDoc.WriteString("Display or change configuration settings for azdo.\n\n")
	longDoc.WriteString("Current respected settings:\n")
	for _, co := range config.Options() {
		longDoc.WriteString(fmt.Sprintf("- %s: %s", co.Key, co.Description))
		if co.DefaultValue != "" {
			longDoc.WriteString(fmt.Sprintf(" (default: %q)", co.DefaultValue))
		}
		longDoc.WriteRune('\n')
	}

	cmd := &cobra.Command{
		Use:   "config <command>",
		Short: "Manage configuration for azdo",
		Long:  longDoc.String(),
	}

	util.DisableAuthCheck(cmd)

	cmd.AddCommand(get.NewCmdConfigGet(ctx))
	cmd.AddCommand(set.NewCmdConfigSet(ctx))
	cmd.AddCommand(list.NewCmdConfigList(ctx))

	return cmd
}
