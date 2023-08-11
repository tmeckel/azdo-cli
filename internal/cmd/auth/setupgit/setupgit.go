package setupgit

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/git"
)

type setupGitOptions struct {
	organizationName string
}

func NewCmdSetupGit(ctx util.CmdContext) *cobra.Command {
	opts := &setupGitOptions{}

	cmd := &cobra.Command{
		Use:   "setup-git",
		Short: "Setup git with AzDO CLI",
		Long: heredoc.Docf(`
			This command configures git to use AzDO CLI as a credential helper.
			For more information on git credential helpers please reference:
			https://git-scm.com/docs/gitcredentials.

			By default, AzDO CLI will be set as the credential helper for all authenticated organizations.
			If there is no authenticated organization the command fails with an error.

			Alternatively, use the %[1]s--organization%[1]s flag to specify a single organization to be configured.
			If the organization is not authenticated with, the command fails with an error.

			Be aware that a credential helper will only work with git remotes that use the HTTPS protocol.
		`, "`"),
		Example: heredoc.Doc(`
			# Configure git to use AzDO CLI as the credential helper for all authenticated organizations
			$ azdo auth setup-git

			# Configure git to use AzDO CLI as the credential helper for a specific organization
			$ azdo auth setup-git --organization enterprise.internal
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setupGitRun(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.organizationName, "organization", "o", "", "Configure git credential helper for specific organization")

	return cmd
}

func setupGitRun(ctx util.CmdContext, opts *setupGitOptions) (err error) {
	cfg, err := ctx.Config()
	if err != nil {
		return
	}
	authCfg := cfg.Authentication()

	organizations := authCfg.GetOrganizations()

	iostrms, err := ctx.IOStreams()
	if err != nil {
		return
	}

	stderr := iostrms.ErrOut
	cs := iostrms.ColorScheme()

	if len(organizations) == 0 {
		fmt.Fprintf(
			stderr,
			"You are not logged into any Azure DevOps organizations. Run %s to authenticate.\n",
			cs.Bold("azdo auth login"),
		)

		return util.ErrSilent
	}

	organizationsToSetup := organizations

	if opts.organizationName != "" {
		if !lo.Contains(organizations, opts.organizationName) {
			fmt.Fprintf(
				stderr,
				"You are not logged the Azure DevOps organization %q. Run %s to authenticate.\n",
				opts.organizationName, cs.Bold("azdo auth login"),
			)
			return util.ErrSilent
		}
		organizationsToSetup = []string{opts.organizationName}
	}

	gitClient, err := ctx.GitClient()
	if err != nil {
		return
	}
	rctx, err := ctx.Context()
	if err != nil {
		return
	}
	for _, organizationName := range organizationsToSetup {

		organizationURL, err := authCfg.GetURL(organizationName)
		if err != nil {
			return err
		}

		// first use a blank value to indicate to git we want to sever the chain of credential helpers
		credHelperKey := fmt.Sprintf("credential.%s.helper", strings.TrimSuffix(organizationURL, "/"))
		preConfigureCmd, err := gitClient.Command(rctx, "config", "--global", "--unset-all", credHelperKey, "")
		if err != nil {
			return err
		}
		if _, err = preConfigureCmd.Output(); err != nil {
			var ge *git.Error
			if !errors.As(err, &ge) || ge.ExitCode != 5 {
				return err
			}
		}

		// second configure the actual helper for this host
		configureCmd, err := gitClient.Command(rctx,
			"config", "--global", "--add",
			credHelperKey,
			fmt.Sprintf("!%s auth git-credential", gitClient.AzDoPath),
		)
		if err != nil {
			return err
		}

		if _, err = configureCmd.Output(); err != nil {
			return err
		}

		configureCmd, err = gitClient.Command(rctx,
			"config", "--global", "--add",
			fmt.Sprintf("%s.useHttpPath", strings.TrimSuffix(credHelperKey, ".helper")),
			"true",
		)
		if err != nil {
			return err
		}

		if _, err = configureCmd.Output(); err != nil {
			return err
		}

		rejectCmd, err := gitClient.Command(rctx, "credential", "reject")
		if err != nil {
			return err
		}

		u, err := url.Parse(organizationURL)
		if err != nil {
			return err
		}
		rejectCmd.Stdin = bytes.NewBufferString(heredoc.Docf(`
			protocol=https
			host=%s
			path=%s
		`, u.Host, u.Path))

		_, err = rejectCmd.Output()
		if err != nil {
			return err
		}

		approveCmd, err := gitClient.Command(rctx, "credential", "approve")
		if err != nil {
			return err
		}

		approveCmd.Stdin = bytes.NewBufferString(heredoc.Docf(`
			protocol=https
			host=%s
			path=%s
			username=%s
			password=%s
		`, u.Host, u.Path, "", ""))

		_, err = approveCmd.Output()
		if err != nil {
			return err
		}

	}

	return nil
}
