package create

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/create/azurerm"
	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/create/github"
	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

type fromFileOptions struct {
	scope    string
	fromFile string
	encoding string
	exporter util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &fromFileOptions{
		encoding: "utf-8",
	}

	cmd := &cobra.Command{
		Use:   "create [ORGANIZATION/]PROJECT --from-file <path> [flags]",
		Short: "Create service connections",
		Long: heredoc.Doc(`
			Create Azure DevOps service endpoints (service connections) from a JSON definition file.

			The project scope accepts the form [ORGANIZATION/]PROJECT. When the organization segment
			is omitted the default organization from configuration is used.

			Check the available subcommands to create service connections of specific well-known types.
		`),
		Example: heredoc.Doc(`
			# Create a service endpoint from a UTF-8 JSON file
			azdo service-endpoint create my-org/my-project --from-file ./endpoint.json

			# Read the definition from stdin using UTF-16LE encoding
			cat endpoint.json | azdo service-endpoint create my-org/my-project --from-file - --encoding utf-16le
		`),
		Aliases: []string{
			"import",
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scope = args[0]
			return runCreateFromFile(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.fromFile, "from-file", "f", "", "Path to the JSON service endpoint definition or '-' for stdin.")
	cmd.Flags().StringVarP(&opts.encoding, "encoding", "e", opts.encoding, "File encoding (utf-8, ascii, utf-16be, utf-16le).")
	util.AddJSONFlags(cmd, &opts.exporter, shared.ServiceEndpointJSONFields)

	_ = cmd.MarkFlagRequired("from-file")

	cmd.AddCommand(azurerm.NewCmd(ctx))

	cmd.AddCommand(github.NewCmd(ctx))

	return cmd
}

func runCreateFromFile(ctx util.CmdContext, opts *fromFileOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	scope, err := util.ParseProjectScope(ctx, opts.scope)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	projectRef, err := shared.ResolveProjectReference(ctx, scope)
	if err != nil {
		return err
	}

	zap.L().Debug("Creating service endpoint from file",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("input", shared.DescribeInput(opts.fromFile)),
		zap.String("encoding", opts.encoding),
	)

	endpoint, err := shared.ReadServiceEndpointFromFile(ios.In, opts.fromFile, opts.encoding)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	// Create specific adjustments:
	// 1. Clear ID (creation always generates a new ID)
	endpoint.Id = nil

	// 2. Strict validation (require name/type/url)
	if err := shared.ValidateEndpointPayload(endpoint, true); err != nil {
		return util.FlagErrorWrap(err)
	}

	// 3. Ensure project references include the current project
	if projectRef != nil {
		if endpoint.ServiceEndpointProjectReferences == nil {
			refs := []serviceendpoint.ServiceEndpointProjectReference{}
			endpoint.ServiceEndpointProjectReferences = &refs
		}
		shared.EnsureProjectReferenceIncluded(endpoint.ServiceEndpointProjectReferences, projectRef)
	}

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

	ios.StopProgressIndicator()

	return shared.Output(ctx, createdEndpoint, opts.exporter)
}
