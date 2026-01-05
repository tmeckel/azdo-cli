package github

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

type githubConfigurer struct {
	cmdCtx          util.CmdContext
	url             string
	token           string
	configurationID string
}

func (g *githubConfigurer) CommandContext() util.CmdContext {
	return g.cmdCtx
}

func (g *githubConfigurer) TypeName() string {
	return "github"
}

func (g *githubConfigurer) Configure(endpoint *serviceendpoint.ServiceEndpoint) error {
	cmdCtx := g.cmdCtx
	ios, err := cmdCtx.IOStreams()
	if err != nil {
		return err
	}

	if g.url == "" {
		g.url = "https://github.com"
	}

	// reuse existing logic from runCreate's auth selection
	if g.token != "" && g.configurationID != "" {
		return fmt.Errorf("--token and --configuration-id are mutually exclusive")
	}

	if g.token == "" && g.configurationID == "" {
		// default to prompting for token when interactive
		if !ios.CanPrompt() {
			return fmt.Errorf("no authentication provided: pass --token or --configuration-id (and enable prompting to provide token interactively)")
		}

		p, err := cmdCtx.Prompter()
		if err != nil {
			return err
		}

		secret, err := p.Password("GitHub token:")
		if err != nil {
			return fmt.Errorf("prompt for token failed: %w", err)
		}
		g.token = secret
	}

	var scheme string
	params := map[string]string{}
	if g.configurationID != "" {
		scheme = "InstallationToken"
		params["ConfigurationId"] = g.configurationID
	} else {
		scheme = "Token"
		params["AccessToken"] = g.token
	}
	endpoint.Url = &g.url
	endpoint.Authorization = &serviceendpoint.EndpointAuthorization{
		Scheme:     &scheme,
		Parameters: &params,
	}
	endpoint.Data = &map[string]string{}
	return nil
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cfg := &githubConfigurer{
		cmdCtx: ctx,
	}

	cmd := &cobra.Command{
		Use:   "github [ORGANIZATION/]PROJECT --name NAME [--url URL] [--token TOKEN]",
		Short: "Create a GitHub service endpoint",
		Long: heredoc.Doc(`
			Create a GitHub service endpoint using a personal access token (PAT) or an installation/oauth configuration.
		`),
		Example: heredoc.Doc(`
			# Create a GitHub service endpoint with a personal access token (PAT)
			azdo service-endpoint create github my-org/my-project --name "gh-ep" --token <PAT>

			# Create a GitHub service endpoint with an installation / OAuth configuration id
			azdo service-endpoint create github my-org/my-project --name "gh-ep" --configuration-id <CONFIG_ID>
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return shared.RunTypedCreate(cmd, args, cfg)
		},
	}

	cmd.Flags().StringVar(&cfg.url, "url", "", "GitHub URL (defaults to https://github.com)")
	// Help text taken from service-endpoint-types.json (inputDescriptors.AccessToken.description)
	cmd.Flags().StringVar(&cfg.token, "token", "", "Visit https://github.com/settings/tokens to create personal access tokens. Recommended scopes: repo, user, admin:repo_hook. If omitted, you will be prompted for a token when interactive.")
	// Support installation/oauth configuration via ConfigurationId (InstallationToken scheme)
	// Help text taken from service-endpoint-types.json (inputDescriptors.ConfigurationId.description)
	cmd.Flags().StringVar(&cfg.configurationID, "configuration-id", "", "Configuration for connecting to the endpoint (use an OAuth/installation configuration). Mutually exclusive with --token.")

	return shared.AddCreateCommonFlags(cmd)
}
