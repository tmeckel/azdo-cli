package github

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type createOptions struct {
	project string

	name            string
	url             string
	token           string
	configurationID string

	exporter util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &createOptions{}

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
			opts.project = args[0]
			return runCreate(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.name, "name", "", "Name of the service endpoint")
	cmd.Flags().StringVar(&opts.url, "url", "", "GitHub URL (defaults to https://github.com)")
	// Help text taken from service-endpoint-types.json (inputDescriptors.AccessToken.description)
	cmd.Flags().StringVar(&opts.token, "token", "", "Visit https://github.com/settings/tokens to create personal access tokens. Recommended scopes: repo, user, admin:repo_hook. If omitted, you will be prompted for a token when interactive.")
	// Support installation/oauth configuration via ConfigurationId (InstallationToken scheme)
	// Help text taken from service-endpoint-types.json (inputDescriptors.ConfigurationId.description)
	cmd.Flags().StringVar(&opts.configurationID, "configuration-id", "", "Configuration for connecting to the endpoint (use an OAuth/installation configuration). Mutually exclusive with --token.")

	_ = cmd.MarkFlagRequired("name")

	util.AddJSONFlags(cmd, &opts.exporter, shared.ServiceEndpointJSONFields)

	return cmd
}

func runCreate(ctx util.CmdContext, opts *createOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	p, err := ctx.Prompter()
	if err != nil {
		return err
	}

	scope, err := util.ParseProjectScope(ctx, opts.project)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	// default URL
	if opts.url == "" {
		opts.url = "https://github.com"
	}

	// authentication selection: token (PAT) or configuration-id (InstallationToken)
	if opts.token != "" && opts.configurationID != "" {
		return fmt.Errorf("--token and --configuration-id are mutually exclusive")
	}
	if opts.token == "" && opts.configurationID == "" {
		// default to prompting for token when interactive
		if !ios.CanPrompt() {
			return fmt.Errorf("no authentication provided: pass --token or --configuration-id (and enable prompting to provide token interactively)")
		}
		secret, err := p.Password("GitHub token:")
		if err != nil {
			return fmt.Errorf("prompt for token failed: %w", err)
		}
		opts.token = secret
	}

	projectRef, err := shared.ResolveProjectReference(ctx, scope)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	endpointType := "github"
	owner := "library"

	var scheme string
	var authParams map[string]string
	if opts.configurationID != "" {
		// InstallationToken scheme expects ConfigurationId parameter
		scheme = "InstallationToken"
		authParams = map[string]string{
			"ConfigurationId": opts.configurationID,
		}
	} else {
		// default to PAT token
		scheme = "Token"
		authParams = map[string]string{
			"AccessToken": opts.token,
		}
	}

	endpoint := &serviceendpoint.ServiceEndpoint{
		Name:  &opts.name,
		Type:  &endpointType,
		Url:   &opts.url,
		Owner: &owner,
		Authorization: &serviceendpoint.EndpointAuthorization{
			Scheme:     &scheme,
			Parameters: &authParams,
		},
		ServiceEndpointProjectReferences: &[]serviceendpoint.ServiceEndpointProjectReference{
			{
				ProjectReference: projectRef,
				Name:             &opts.name,
				Description:      types.ToPtr(""),
			},
		},
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	client, err := ctx.ClientFactory().ServiceEndpoint(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create service endpoint client: %w", err)
	}

	createdEndpoint, err := client.CreateServiceEndpoint(ctx.Context(), serviceendpoint.CreateServiceEndpointArgs{
		Endpoint: endpoint,
	})
	if err != nil {
		return fmt.Errorf("failed to create service endpoint: %w", err)
	}

	zap.L().Debug("github service endpoint created",
		zap.String("id", types.GetValue(createdEndpoint.Id, uuid.Nil).String()),
		zap.String("name", types.GetValue(createdEndpoint.Name, "")),
	)

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		shared.RedactSecrets(createdEndpoint)
	}

	return shared.Output(ctx, createdEndpoint, opts.exporter)
}
