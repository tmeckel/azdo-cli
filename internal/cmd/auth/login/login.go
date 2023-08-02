package login

import (
	"io"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

type loginOptions struct {
	MainExecutable  string
	Interactive     bool
	OrganizationUrl string
	Token           string
	GitProtocol     string
	InsecureStorage bool
}

func NewCmdLogin(ctx util.CmdContext) *cobra.Command {
	var tokenStdin bool

	opts := &loginOptions{}

	cmd := &cobra.Command{
		Use:   "login",
		Args:  cobra.ExactArgs(0),
		Short: "Authenticate with a Azure DevOps oragnization",
		Long: heredoc.Docf(`
			Authenticate with a Azure DevOps Organization.

			The default authentication mode is a web-based browser flow. After completion, an
			authentication token will be stored internally.

			Alternatively, use %[1]s--with-token%[1]s to pass in a token on standard input.
			The minimum required scopes for the token are: "repo", "read:org".

			Alternatively, azdo will use the authentication token (PAT) found in environment variables.
			This method is most suitable for "headless" use of azdo such as in automation. See
			%[1]sazdo help environment%[1]s for more info.

			To use azdo in Azure DevOps Pipeline Tasks (or other automation environments), add %[1]sAZDO_TOKEN: ${{ azdo.token }}%[1]s to "env".
		`, "`"),
		Example: heredoc.Doc(`
		# start interactive setup
		$ azdo auth login

		# authenticate by reading the token from a file
		$ azdo auth login --with-token < mytoken.txt

		# authenticate with a specific Azure DevOps Organization
		$ azdo auth login --organizationUrl https://dev.azure.com/myorg
	`),
		RunE: func(cmd *cobra.Command, args []string) error {
			iostreams, err := ctx.IOStreams()
			if err != nil {
				return util.FlagErrorf("error getting io streams: %w", err)
			}

			if tokenStdin {
				defer iostreams.In.Close()
				token, err := io.ReadAll(iostreams.In)
				if err != nil {
					return util.FlagErrorf("failed to read token from standard input: %w", err)
				}
				opts.Token = strings.TrimSpace(string(token))
			}

			if iostreams.CanPrompt() && opts.Token == "" {
				opts.Interactive = true
			}

			return loginRun(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.OrganizationUrl, "organizationUrl", "o", "", "The URL to the Azure DevOps organization to authenticate with")
	cmd.Flags().BoolVar(&tokenStdin, "with-token", false, "Read token from standard input")
	util.StringEnumFlag(cmd, &opts.GitProtocol, "git-protocol", "p", "", []string{"ssh", "https"}, "The protocol to use for git operations")
	cmd.Flags().BoolVar(&opts.InsecureStorage, "insecure-storage", false, "Save authentication credentials in plain text instead of credential store")

	return cmd
}

func loginRun(ctx util.CmdContext, opts *loginOptions) (err error) {
	cfg, err := ctx.Config()
	if err != nil {
		return util.FlagErrorf("error getting io configuration: %w", err)
	}
	p, err := ctx.Prompter()
	if err != nil {
		return util.FlagErrorf("error getting io propter: %w", err)
	}

	organizationUrl := opts.OrganizationUrl
	organizationName := ""

	if opts.Interactive && organizationUrl == "" {
		organizationUrl, organizationName, err = promptForOrganizationName(ctx, opts)
		if err != nil {
			return
		}
	}

	gitProtocol := strings.ToLower(opts.GitProtocol)
	if opts.Interactive && gitProtocol == "" {
		options := []string{
			"HTTPS",
			"SSH",
		}
		result, err := p.Select(
			"What is your preferred protocol for Git operations?",
			options[0],
			options)
		if err != nil {
			return err
		}
		proto := options[result]
		gitProtocol = strings.ToLower(proto)
	}

	authToken := opts.Token
	if opts.Token == "" {
		authToken, err = p.AuthToken()
		if err != nil {
			return
		}
	}

	authCfg := cfg.Authentication()
	if err = authCfg.Login(organizationName, organizationUrl, authToken, gitProtocol, !opts.InsecureStorage); err != nil {
		return
	}

	return
}

func promptForOrganizationName(ctx util.CmdContext, opts *loginOptions) (organizationUrl string, organizationName string, err error) {
	options := []string{"https://dev.azure.com/{organization}", "https://{organization}.visualstudio.com"}
	p, err := ctx.Prompter()
	if err != nil {
		return
	}
	orgType, err := p.Select(
		"Azure DevOps Organization URL type?",
		options[0],
		options)
	if err != nil {
		return
	}

	organizationName, err = p.InputOrganizationName()
	if err != nil {
		return
	}

	organizationName = strings.ToLower(organizationName)
	organizationUrl = strings.Replace(options[orgType], "{organization}", organizationName, -1)

	return
}
