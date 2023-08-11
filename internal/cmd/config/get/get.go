package get

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/config"
)

type getOptions struct {
	OrganizationName string
	Key              string
}

func NewCmdConfigGet(ctx util.CmdContext) *cobra.Command {
	opts := &getOptions{}

	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Print the value of a given configuration key",
		Example: heredoc.Doc(`
			$ azdo config get git_protocol
			https
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Key = args[0]

			return getRun(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.OrganizationName, "organization", "o", "", "Get per-organization setting")

	return cmd
}

func getRun(ctx util.CmdContext, opts *getOptions) (err error) {
	cfg, err := ctx.Config()
	if err != nil {
		return util.FlagErrorf("error getting io configuration: %w", err)
	}
	iostreams, err := ctx.IOStreams()
	if err != nil {
		return util.FlagErrorf("error getting io streams: %w", err)
	}

	// search keyring storage when fetching the `oauth_token` value
	if opts.OrganizationName != "" && opts.Key == "pat" {
		token, err := cfg.Authentication().GetToken(opts.OrganizationName)
		if err != nil {
			return util.FlagErrorf("failed to get token for organization %s; %w", opts.OrganizationName, err)
		}
		fmt.Fprintf(iostreams.Out, "%s\n", token)
		return nil
	}

	keys := []string{}
	if opts.OrganizationName != "" {
		keys = append(keys, config.Organizations, opts.OrganizationName)
	}
	keys = append(keys, opts.Key)
	val, err := cfg.GetOrDefault(keys)
	if err != nil {
		return err
	}

	if val != "" {
		fmt.Fprintf(iostreams.Out, "%s\n", val)
	}
	return nil
}
