package update

import (
	"errors"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	targetArg string

	name        string
	description string
	url         string
	fromFile    string
	encoding    string
	nameChanged bool
	descChanged bool
	urlChanged  bool

	enableForAll     bool
	enableForAllUsed bool

	exporter util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "update [ORGANIZATION/]PROJECT/ID_OR_NAME",
		Short: "Update a service endpoint.",
		Long: heredoc.Doc(`
			Update an existing Azure DevOps service endpoint (service connection).

			The positional argument accepts the form [ORGANIZATION/]PROJECT/ID_OR_NAME. When the
			organization segment is omitted the default organization from configuration is used.

			Provide one or more mutating flags to change attributes or pipeline permissions.
		`),
		Args: util.ExactArgs(1, "service endpoint target required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.targetArg = args[0]
			o.enableForAllUsed = cmd.Flags().Changed("enable-for-all")
			o.nameChanged = cmd.Flags().Changed("name")
			o.descChanged = cmd.Flags().Changed("description")
			o.urlChanged = cmd.Flags().Changed("url")
			return run(ctx, o)
		},
	}

	cmd.Flags().StringVar(&o.name, "name", "", "New friendly name for the service endpoint.")
	cmd.Flags().StringVar(&o.description, "description", "", "New description for the service endpoint.")
	cmd.Flags().StringVar(&o.url, "url", "", "New service endpoint URL.")
	cmd.Flags().StringVarP(&o.fromFile, "from-file", "f", "", "Path to a JSON service endpoint definition or '-' for stdin. Mutually exclusive with --name/--description/--url.")
	cmd.Flags().StringVarP(&o.encoding, "encoding", "e", "utf-8", "File encoding (utf-8, ascii, utf-16be, utf-16le).")
	cmd.Flags().BoolVar(&o.enableForAll, "enable-for-all", false, "Grant (true) or revoke (false) access for all pipelines.")

	util.AddJSONFlags(cmd, &o.exporter, shared.ServiceEndpointJSONFields)

	return cmd
}

func run(ctx util.CmdContext, o *opts) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	scope, err := util.ParseProjectTargetWithDefaultOrganization(ctx, o.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	fromFileSet := strings.TrimSpace(o.fromFile) != ""

	if !(o.nameChanged || o.descChanged || o.urlChanged || fromFileSet || o.enableForAllUsed) {
		return util.FlagErrorf("at least one mutating flag must be supplied")
	}

	if fromFileSet && (o.nameChanged || o.descChanged || o.urlChanged) {
		return util.FlagErrorf("--from-file is mutually exclusive with --name, --description, and --url")
	}

	client, err := ctx.ClientFactory().ServiceEndpoint(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create service endpoint client: %w", err)
	}

	endpoint, err := shared.FindServiceEndpoint(ctx, client, scope.Project, scope.Target)
	if err != nil {
		if errors.Is(err, shared.ErrEndpointNotFound) {
			ios.StopProgressIndicator()
			cs := ios.ColorScheme()
			fmt.Fprintf(ios.Out, "%s Service endpoint %q was not found in %s/%s.\n", cs.WarningIcon(), scope.Target, scope.Organization, scope.Project)
			return nil
		}
		return err
	}
	if endpoint == nil || endpoint.Id == nil {
		return fmt.Errorf("resolved service endpoint %q is missing an identifier", scope.Target)
	}

	var cachedProjectRef *serviceendpoint.ProjectReference
	getProjectRef := func() (*serviceendpoint.ProjectReference, error) {
		if cachedProjectRef != nil {
			return cachedProjectRef, nil
		}
		ref, err := shared.ResolveProjectReference(ctx, &scope.Scope)
		if err != nil {
			return nil, err
		}
		cachedProjectRef = ref
		return cachedProjectRef, nil
	}

	var toUpdate *serviceendpoint.ServiceEndpoint

	if fromFileSet {
		// Parse from file using shared helper (performs baseline validation)
		payload, err := shared.ReadServiceEndpointFromFile(ios.In, o.fromFile, o.encoding)
		if err != nil {
			return util.FlagErrorWrap(err)
		}

		// Force the ID to match the target endpoint
		payload.Id = endpoint.Id

		// Merge missing fields from the existing endpoint
		// This allows the file to be partial (e.g. just updating description or auth)
		if payload.Name == nil {
			payload.Name = endpoint.Name
		}
		if payload.Type == nil {
			payload.Type = endpoint.Type
		}
		if payload.Url == nil {
			payload.Url = endpoint.Url
		}
		if payload.Authorization == nil {
			payload.Authorization = endpoint.Authorization
		}
		if payload.Data == nil {
			payload.Data = endpoint.Data
		}

		// Now perform strict validation on the effective payload
		if err := shared.ValidateEndpointPayload(payload, true); err != nil {
			return util.FlagErrorWrap(err)
		}

		projectRef, err := getProjectRef()
		if err != nil {
			return err
		}

		// Ensure the project references list is initialized and includes the current project
		if payload.ServiceEndpointProjectReferences == nil {
			refs := []serviceendpoint.ServiceEndpointProjectReference{}
			payload.ServiceEndpointProjectReferences = &refs
		}
		shared.EnsureProjectReferenceIncluded(payload.ServiceEndpointProjectReferences, projectRef)

		toUpdate = payload
	} else {
		copy := *endpoint
		if o.nameChanged {
			copy.Name = types.ToPtr(o.name)
		}
		if o.descChanged {
			copy.Description = types.ToPtr(o.description)
		}
		if o.urlChanged {
			copy.Url = types.ToPtr(o.url)
		}

		toUpdate = &copy
	}

	fields := []zap.Field{
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("identifier", scope.Target),
		zap.String("endpointId", types.GetValue(toUpdate.Id, uuid.Nil).String()),
	}

	if fromFileSet {
		fields = append(fields,
			zap.String("mode", "from-file"),
			zap.String("input", shared.DescribeInput(o.fromFile)),
			zap.String("encoding", o.encoding),
		)
	} else {
		fields = append(fields, zap.String("mode", "overlay"))
	}

	zap.L().Debug("Updating service endpoint", fields...)

	updated, err := client.UpdateServiceEndpoint(ctx.Context(), serviceendpoint.UpdateServiceEndpointArgs{
		Endpoint:   toUpdate,
		EndpointId: toUpdate.Id,
	})
	if err != nil {
		return fmt.Errorf("failed to update service endpoint: %w", err)
	}

	if o.enableForAllUsed {
		projectRef, err := getProjectRef()
		if err != nil {
			return err
		}
		projectID := types.GetValue(projectRef.Id, uuid.Nil)
		if projectID == uuid.Nil {
			return fmt.Errorf("project reference missing ID")
		}

		endpointID := types.GetValue(updated.Id, uuid.Nil)
		if endpointID == uuid.Nil {
			return fmt.Errorf("updated service endpoint is missing an ID")
		}

		if err := shared.SetAllPipelinesAccessToEndpoint(ctx,
			scope.Organization,
			projectID,
			endpointID,
			o.enableForAll,
			nil,
		); err != nil {
			return fmt.Errorf("failed to update pipeline permissions for endpoint: %w", err)
		}
	}

	ios.StopProgressIndicator()

	return shared.Output(ctx, updated, o.exporter)
}
