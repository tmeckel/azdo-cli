package show

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/template"
)

//go:embed show.tpl
var showTpl string

type showOptions struct {
	targetArg string
	exporter  util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &showOptions{}

	cmd := &cobra.Command{
		Use:   "show [ORGANIZATION/]PROJECT/ID_OR_NAME",
		Short: "Show details of a service endpoint.",
		Long: heredoc.Doc(`
			Show details of a single Azure DevOps service endpoint (service connection).

			The positional argument accepts the form [ORGANIZATION/]PROJECT/ID_OR_NAME. When the
			organization segment is omitted the default organization from configuration is used.
		`),
		Example: heredoc.Doc(`
			# Show a service endpoint by ID in the default organization
			azdo service-endpoint show MyProject/12345678-1234-1234-1234-1234567890ab

			# Show a service endpoint by name in a specific organization
			azdo service-endpoint show myorg/MyProject/MyConnection
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runShow(ctx, opts)
		},
	}

	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"administratorsGroup",
		"authorization",
		"createdBy",
		"data",
		"description",
		"groupScopeId",
		"id",
		"isReady",
		"isShared",
		"name",
		"operationStatus",
		"owner",
		"readersGroup",
		"serviceEndpointProjectReferences",
		"type",
		"url",
	})

	return cmd
}

func runShow(ctx util.CmdContext, opts *showOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	scope, err := util.ParseProjectTargetWithDefaultOrganization(ctx, opts.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
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

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, endpoint)
	}

	t := template.New(
		ios.Out,
		ios.TerminalWidth(),
		ios.ColorEnabled()).
		WithTheme(ios.TerminalTheme()).
		WithFuncs(map[string]any{
			"s": func(v *string) string {
				if v == nil {
					return ""
				}
				return *v
			},
			"hasText": func(v *string) bool {
				if v == nil {
					return false
				}
				return strings.TrimSpace(*v) != ""
			},
			"b": func(v *bool) string {
				if v == nil {
					return ""
				}
				return fmt.Sprintf("%v", *v)
			},
			"u": func(v *uuid.UUID) string {
				if v == nil {
					return ""
				}
				return v.String()
			},
			"scheme": func(ep *serviceendpoint.EndpointAuthorization) string {
				if ep == nil || ep.Scheme == nil {
					return ""
				}
				return *ep.Scheme
			},
			"identity": func(id *webapi.IdentityRef) string {
				if id == nil || id.DisplayName == nil {
					return ""
				}
				return fmt.Sprintf("%s (%s)", *id.DisplayName, *id.UniqueName)
			},
		})
	err = t.Parse(showTpl)
	if err != nil {
		return err
	}

	return t.ExecuteData(endpoint)
}
