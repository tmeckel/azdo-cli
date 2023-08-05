package list

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/config"
)

type listOptions struct {
	organizationName string
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

	return cmd
}

func listRun(ctx util.CmdContext, opts *listOptions) error {
	cfg, err := ctx.Config()
	if err != nil {
		return util.FlagErrorf("error getting io configuration: %w", err)
	}
	iostreams, err := ctx.IOStreams()
	if err != nil {
		return util.FlagErrorf("error getting io streams: %w", err)
	}

	var host string
	if opts.organizationName != "" {
		host = opts.organizationName
	} else {
		host, _ = cfg.Authentication().GetDefaultOrganization()
	}

	configOptions := config.Options()

	var keys []string
	if host != "" {
		keys = make([]string, 3)
		keys = append(keys, config.Organizations, host)
	} else {
		keys = make([]string, 1)
	}

	for _, key := range configOptions {
		keys[len(keys)-1] = key.Key
		val, err := cfg.GetOrDefault(keys)
		if err != nil {
			return err
		}
		fmt.Fprintf(iostreams.Out, "%s=%s\n", key.Key, val)
	}

	return nil
}
