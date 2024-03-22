package get

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/pterm/pterm"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/config"
)

type getOptions struct {
	organizationName string
	key              string
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
			opts.key = args[0]

			return getRun(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.organizationName, "organization", "o", "", "Get per-organization setting")

	return cmd
}

func getRun(ctx util.CmdContext, opts *getOptions) (err error) {
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

	// search keyring storage when fetching the `oauth_token` value
	if opts.organizationName != "" && opts.key == "pat" {
		token, err := cfg.Authentication().GetToken(opts.organizationName)
		if err != nil {
			return util.FlagErrorf("failed to get token for organization %s; %w", opts.organizationName, err)
		}
		fmt.Fprintf(iostrms.Out, "%s\n", token)
		return nil
	}

	keys := []string{}
	if opts.organizationName != "" {
		keys = append(keys, config.Organizations, opts.organizationName)
	}
	keys = append(keys, opts.key)
	val, err := cfg.GetOrDefault(keys)
	if err != nil {
		return err
	}

	if val != "" {
		fmt.Fprintf(iostrms.Out, "%s\n", val)
	}
	return nil
}
