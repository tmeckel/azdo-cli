package logout

import (
	"errors"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/config"
	"github.com/tmeckel/azdo-cli/internal/git"
	"go.uber.org/zap"
)

type logoutOptions struct {
	organizationName string
}

func NewCmdLogout(ctx util.CmdContext) *cobra.Command {
	opts := &logoutOptions{}

	cmd := &cobra.Command{
		Use:   "logout",
		Args:  cobra.ExactArgs(0),
		Short: "Log out of a Azure DevOps organization",
		Long: heredoc.Docf(`Remove authentication for a Azure DevOps organization.

			This command removes the authentication configuration for an organization either specified
			interactively or via %[1]s--organization%[1]s.
		`, "`"),
		Example: heredoc.Doc(`
			$ azdo auth logout
			# => select what organization to log out of via a prompt

			$ azdo auth logout --hostname enterprise.internal
			# => log out of specified organization
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			iostrms, err := ctx.IOStreams()
			if err != nil {
				return err
			}

			if opts.organizationName == "" && !iostrms.CanPrompt() {
				return util.FlagErrorf("--organization required when not running interactively")
			}

			return logoutRun(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.organizationName, "organization", "o", "", "The Azure DevOps organization to log out of")

	return cmd
}

func logoutRun(ctx util.CmdContext, opts *logoutOptions) (err error) {
	logger := zap.L().Sugar()

	cfg, err := ctx.Config()
	if err != nil {
		return util.FlagErrorf("error getting io configuration: %w", err)
	}
	iostrms, err := ctx.IOStreams()
	if err != nil {
		return
	}
	p, err := ctx.Prompter()
	if err != nil {
		return fmt.Errorf("error getting io propter: %w", err)
	}

	cs := iostrms.ColorScheme()
	authCfg := cfg.Authentication()

	organizations := authCfg.GetOrganizations()
	organizationName := opts.organizationName

	if len(organizations) == 0 {
		fmt.Fprintf(
			iostrms.ErrOut,
			"You are %s logged into any Azure DevOps organizations.\n",
			cs.Red("not"),
		)

		return util.ErrSilent
	}

	if organizationName == "" {
		if len(organizations) == 1 {
			organizationName = organizations[0]
		} else {
			selected, err := p.Select(
				"What organization do you want to log out of?", "", organizations)
			if err != nil {
				return fmt.Errorf("could not prompt: %w", err)
			}
			organizationName = organizations[selected]
		}
	} else {
		if !lo.Contains(organizations, opts.organizationName) {
			fmt.Fprintf(
				iostrms.ErrOut,
				"You are %s logged in to the Azure DevOps organization %q.\n",
				cs.Red("not"),
				opts.organizationName,
			)
			return util.ErrSilent
		}
	}

	// Logout must
	// 1. Check if the organization set as default, if yes: clear default
	defaultOrganization, err := authCfg.GetDefaultOrganization()
	if err != nil {
		return err
	}
	if defaultOrganization == organizationName {
		result, err := p.Confirm(fmt.Sprintf("%q is the current default organization. Perform logout?", organizationName), false)
		if err != nil {
			return err
		}
		if !result {
			return nil
		}
		err = authCfg.SetDefaultOrganization("")
		if err != nil {
			if !errors.Is(err, &config.KeyNotFoundError{}) {
				return err
			}
		}
	}
	// 2. Remove global credential helper (azdo auth setup-git) if it exists
	organizationURL, err := authCfg.GetURL(organizationName)
	if err != nil {
		return err
	}

	rctx, err := ctx.Context()
	if err != nil {
		return
	}

	gitClient, err := ctx.GitClient()
	if err != nil {
		return
	}

	credHelperKey := fmt.Sprintf("credential.%s", strings.TrimSuffix(organizationURL, "/"))
	preConfigureCmd, err := gitClient.Command(rctx, "config", "--global", "--remove-section", credHelperKey)
	if err != nil {
		return err
	}
	if _, err = preConfigureCmd.Output(); err != nil {
		logger.Debugf("failed to execute command. Error type %T; %+v", err, err)

		var ge *git.Error
		if !errors.As(err, &ge) || (ge.ExitCode != 5 && ge.ExitCode != 128) {
			return err
		}
	}

	// 3. Remove the organization from the config
	return authCfg.Logout(organizationName)
}
