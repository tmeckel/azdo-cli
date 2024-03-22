package list

import (
	"fmt"

	"github.com/pterm/pterm"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/config"
)

type listOptions struct {
	organizationName string
	all              bool
}

func NewCmdConfigList(ctx util.CmdContext) *cobra.Command {
	opts := &listOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "Print a list of configuration keys and values",
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.organizationName, "organization", "o", "", "Get per-organization configuration")
	cmd.Flags().BoolVar(&opts.all, "all", false, "Show config options which are not configured")
	return cmd
}

func listRun(ctx util.CmdContext, opts *listOptions) error {
	cfg, err := ctx.Config()
	if err != nil {
		return util.FlagErrorf("error getting io configuration: %w", err)
	}
	iostrms, err := ctx.IOStreams()
	if err != nil {
		return util.FlagErrorf("error getting io streams: %w", err)
	}

	if opts.organizationName != "" {
		if !lo.Contains(cfg.Authentication().GetOrganizations(), opts.organizationName) {
			fmt.Fprintf(
				iostrms.ErrOut,
				"You are not logged the Azure DevOps organization %q. Run %s to authenticate.\n",
				opts.organizationName, pterm.Bold.Sprint("azdo auth login"),
			)
			return util.ErrSilent
		}
	}

	configOptions := config.Options()

	var keys []string
	if opts.organizationName != "" {
		keys = make([]string, 3)
		keys[0] = config.Organizations
		keys[1] = opts.organizationName
	} else {
		keys = make([]string, 1)
	}

	for _, key := range configOptions {
		keys[len(keys)-1] = key.Key
		val, err := cfg.GetOrDefault(keys)
		if err != nil {
			return err
		}
		if val != "" || opts.all {
			fmt.Fprintf(iostrms.Out, "%s=%s\n", key.Key, val)
		}
	}

	return nil
}
