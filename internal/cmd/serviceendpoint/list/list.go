package list

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/emirpasic/gods/sets/hashset"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type listOptions struct {
	scopeArg         string
	organization     string
	project          string
	typeFilter       string
	owner            string
	authSchemes      []string
	endpointIDValues []string
	actionFilter     string
	includeFailed    bool
	includeDetails   bool
	nameFilters      []string
	outputFormat     string
	exporter         util.Exporter
}

var actionFilterMap = map[string]serviceendpoint.ServiceEndpointActionFilter{
	"manage": serviceendpoint.ServiceEndpointActionFilterValues.Manage,
	"use":    serviceendpoint.ServiceEndpointActionFilterValues.Use,
	"view":   serviceendpoint.ServiceEndpointActionFilterValues.View,
	"none":   serviceendpoint.ServiceEndpointActionFilterValues.None,
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &listOptions{
		outputFormat: "table",
	}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION/]PROJECT",
		Short: "List service endpoints in a project.",
		Long: heredoc.Doc(`
			List Azure DevOps service endpoints (service connections) defined within a project.

			The project scope accepts the form [ORGANIZATION/]PROJECT. When the organization
			segment is omitted the default organization from configuration is used.
		`),
		Example: heredoc.Doc(`
			# List service endpoints for a project using the default organization
			azdo service-endpoint list MyProject

			# List service endpoints for a project in a specific organization
			azdo service-endpoint list myorg/MyProject

			# List AzureRM endpoints that are ready for use
			azdo service-endpoint list myorg/MyProject --type AzureRM --action-filter manage
		`),
		Aliases: []string{"ls", "l"},
		Args:    util.ExactArgs(1, "project argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scopeArg = args[0]
			return runList(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.typeFilter, "type", "", "Filter by service endpoint type (e.g., AzureRM, GitHub, Generic).")
	cmd.Flags().StringVar(&opts.owner, "owner", "", "Filter by service endpoint owner (e.g., Library, AgentCloud).")
	cmd.Flags().StringSliceVar(&opts.authSchemes, "auth-scheme", nil, "Filter by authorization scheme. Repeat to specify multiple values or separate multiple values by comma ','.")
	cmd.Flags().StringSliceVar(&opts.endpointIDValues, "endpoint-id", nil, "Filter by endpoint ID (UUID). Repeat to specify multiple values or separate multiple values by comma ','.")
	cmd.Flags().StringVar(&opts.actionFilter, "action-filter", "", "Filter endpoints by caller permissions (manage, use, view, none).")
	cmd.Flags().BoolVar(&opts.includeFailed, "include-failed", false, "Include endpoints that are in a failed state.")
	cmd.Flags().BoolVar(&opts.includeDetails, "include-details", false, "Request additional authorization metadata when available.")
	cmd.Flags().StringSliceVar(&opts.nameFilters, "name", nil, "Filter by endpoint display name. Repeat to specify multiple values or separate multiple values by comma ','.")
	util.StringEnumFlag(cmd, &opts.outputFormat, "output-format", "", "table", []string{"table", "ids"}, "Select non-JSON output format")
	util.AddJSONFlags(cmd, &opts.exporter, shared.ServiceEndpointJSONFields)

	return cmd
}

func runList(ctx util.CmdContext, opts *listOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	progressStopped := false
	defer func() {
		if !progressStopped {
			ios.StopProgressIndicator()
		}
	}()

	scope, err := util.ParseProjectScope(ctx, opts.scopeArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}
	opts.organization = scope.Organization
	opts.project = scope.Project

	authSchemes := normalizeSlice(opts.authSchemes)
	names := normalizeSlice(opts.nameFilters)

	var endpointIDs []uuid.UUID
	if len(opts.endpointIDValues) > 0 {
		endpointIDs, err = parseUUIDs(opts.endpointIDValues)
		if err != nil {
			return err
		}
	}

	var actionFilter *serviceendpoint.ServiceEndpointActionFilter
	if strings.TrimSpace(opts.actionFilter) != "" {
		filterValue := strings.ToLower(strings.TrimSpace(opts.actionFilter))
		value, ok := actionFilterMap[filterValue]
		if !ok {
			return util.FlagErrorf("invalid action filter %q: valid values are {manage|use|view|none}", opts.actionFilter)
		}
		actionFilter = types.ToPtr(value)
	}

	zap.L().Debug("Listing service endpoints",
		zap.String("organization", opts.organization),
		zap.String("project", opts.project),
		zap.String("type", strings.TrimSpace(opts.typeFilter)),
		zap.String("owner", strings.TrimSpace(opts.owner)),
		zap.Strings("authSchemes", authSchemes),
		zap.Int("endpointIDCount", len(endpointIDs)),
		zap.Strings("names", names),
		zap.Bool("includeFailed", opts.includeFailed),
		zap.Bool("includeDetails", opts.includeDetails),
		zap.String("actionFilter", strings.TrimSpace(opts.actionFilter)),
	)

	client, err := ctx.ClientFactory().ServiceEndpoint(ctx.Context(), opts.organization)
	if err != nil {
		return fmt.Errorf("failed to create service endpoint client: %w", err)
	}

	var endpoints []serviceendpoint.ServiceEndpoint
	if len(names) > 0 {
		endpoints, err = listByNames(ctx, client, opts, names, authSchemes)
		if err != nil {
			return err
		}
		if len(endpointIDs) > 0 {
			endpoints = intersectByID(endpoints, endpointIDs)
			if len(endpoints) == 0 {
				return util.NewNoResultsError("no service endpoints matched the provided --name and --endpoint-id filters")
			}
		}
	} else {
		endpoints, err = listAll(ctx, client, opts, authSchemes, endpointIDs)
		if err != nil {
			return err
		}
	}

	if actionFilter != nil {
		endpoints, err = filterByAction(ctx, client, opts.project, actionFilter, endpoints)
		if err != nil {
			return err
		}
		if len(endpoints) == 0 {
			return util.NewNoResultsError("no service endpoints matched the requested action filter")
		}
	}

	if len(endpoints) == 0 {
		return util.NewNoResultsError(fmt.Sprintf("no service endpoints found for project %s in organization %s", opts.project, opts.organization))
	}

	sort.Slice(endpoints, func(i, j int) bool {
		return strings.ToLower(types.GetValue(endpoints[i].Name, "")) < strings.ToLower(types.GetValue(endpoints[j].Name, ""))
	})

	ios.StopProgressIndicator()
	progressStopped = true

	if opts.exporter != nil {
		return opts.exporter.Write(ios, endpoints)
	}

	switch opts.outputFormat {
	case "ids":
		for _, ep := range endpoints {
			if ep.Id != nil {
				fmt.Fprintln(ios.Out, ep.Id.String())
			}
		}
		return nil
	default:
		return renderTable(ctx, endpoints)
	}
}

func listAll(ctx util.CmdContext, client serviceendpoint.Client, opts *listOptions, authSchemes []string, endpointIDs []uuid.UUID) ([]serviceendpoint.ServiceEndpoint, error) {
	args := serviceendpoint.GetServiceEndpointsArgs{
		Project: types.ToPtr(opts.project),
	}

	if strings.TrimSpace(opts.typeFilter) != "" {
		args.Type = types.ToPtr(strings.TrimSpace(opts.typeFilter))
	}
	if len(authSchemes) > 0 {
		args.AuthSchemes = types.ToPtr(authSchemes)
	}
	if len(endpointIDs) > 0 {
		args.EndpointIds = types.ToPtr(endpointIDs)
	}
	if strings.TrimSpace(opts.owner) != "" {
		args.Owner = types.ToPtr(strings.TrimSpace(opts.owner))
	}
	if opts.includeFailed {
		args.IncludeFailed = types.ToPtr(true)
	}
	if opts.includeDetails {
		args.IncludeDetails = types.ToPtr(true)
	}

	result, err := client.GetServiceEndpoints(ctx.Context(), args)
	if err != nil {
		return nil, fmt.Errorf("failed to list service endpoints: %w", err)
	}
	if result == nil {
		return []serviceendpoint.ServiceEndpoint{}, nil
	}
	return *result, nil
}

func listByNames(ctx util.CmdContext, client serviceendpoint.Client, opts *listOptions, names, authSchemes []string) ([]serviceendpoint.ServiceEndpoint, error) {
	nameCopy := append([]string(nil), names...)

	args := serviceendpoint.GetServiceEndpointsByNamesArgs{
		Project:       types.ToPtr(opts.project),
		EndpointNames: &nameCopy,
	}

	if strings.TrimSpace(opts.typeFilter) != "" {
		args.Type = types.ToPtr(strings.TrimSpace(opts.typeFilter))
	}
	if len(authSchemes) > 0 {
		args.AuthSchemes = types.ToPtr(authSchemes)
	}
	if strings.TrimSpace(opts.owner) != "" {
		args.Owner = types.ToPtr(strings.TrimSpace(opts.owner))
	}
	if opts.includeFailed {
		args.IncludeFailed = types.ToPtr(true)
	}
	if opts.includeDetails {
		args.IncludeDetails = types.ToPtr(true)
	}

	result, err := client.GetServiceEndpointsByNames(ctx.Context(), args)
	if err != nil {
		return nil, fmt.Errorf("failed to list service endpoints by name: %w", err)
	}
	if result == nil {
		return []serviceendpoint.ServiceEndpoint{}, nil
	}
	return *result, nil
}

func filterByAction(ctx util.CmdContext, client serviceendpoint.Client, project string, action *serviceendpoint.ServiceEndpointActionFilter, endpoints []serviceendpoint.ServiceEndpoint) ([]serviceendpoint.ServiceEndpoint, error) {
	filtered := make([]serviceendpoint.ServiceEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep.Id == nil {
			continue
		}
		sed, err := client.GetServiceEndpointDetails(ctx.Context(), serviceendpoint.GetServiceEndpointDetailsArgs{
			Project:      types.ToPtr(project),
			EndpointId:   ep.Id,
			ActionFilter: action,
		})
		if err != nil {
			if isForbidden(err) {
				continue
			}
			fallback := ""
			if ep.Id != nil {
				fallback = ep.Id.String()
			}
			return nil, fmt.Errorf("failed to fetch permissions for endpoint %s: %w", types.GetValue(ep.Name, fallback), err)
		}
		if sed.Id == nil { // GetServiceEndpointDetails returns an empty result instead of nil or an HTTP 404
			continue
		}
		filtered = append(filtered, ep)
	}
	return filtered, nil
}

func renderTable(ctx util.CmdContext, endpoints []serviceendpoint.ServiceEndpoint) error {
	tp, err := ctx.Printer("table")
	if err != nil {
		return err
	}

	tp.AddColumns("ID", "Name", "Type", "Owner", "Ready", "Shared", "Auth Scheme", "Project Reference")

	for _, ep := range endpoints {
		scope := shared.FormatProjectReference(ep.ServiceEndpointProjectReferences)

		id := ""
		if ep.Id != nil {
			id = ep.Id.String()
		}

		tp.AddField(id, printer.WithTruncate(nil))
		tp.AddField(types.GetValue(ep.Name, ""))
		tp.AddField(types.GetValue(ep.Type, ""))
		tp.AddField(types.GetValue(ep.Owner, ""))
		tp.AddField(formatBool(types.GetValue(ep.IsReady, false)))
		tp.AddField(formatBool(types.GetValue(ep.IsShared, false)))
		tp.AddField(shared.AuthorizationScheme(&ep))
		tp.AddField(scope)
		tp.EndRow()
	}

	return tp.Render()
}

func normalizeSlice(values []string) []string {
	if len(values) == 0 {
		return values
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseUUIDs(raw []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(raw))
	for _, val := range raw {
		trimmed := strings.TrimSpace(val)
		if trimmed == "" {
			return nil, util.FlagErrorf("endpoint id cannot be empty")
		}
		parsed, err := uuid.Parse(trimmed)
		if err != nil {
			return nil, util.FlagErrorf("invalid endpoint id %q: %v", val, err)
		}
		out = append(out, parsed)
	}
	return out, nil
}

func intersectByID(endpoints []serviceendpoint.ServiceEndpoint, ids []uuid.UUID) []serviceendpoint.ServiceEndpoint {
	if len(ids) == 0 {
		return endpoints
	}
	idSet := hashset.New(ids)
	filtered := make([]serviceendpoint.ServiceEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep.Id == nil {
			continue
		}
		if ok := idSet.Contains(*ep.Id); ok {
			filtered = append(filtered, ep)
		}
	}
	return filtered
}

func formatBool(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
}

func isForbidden(err error) bool {
	var wrapped *azuredevops.WrappedError
	if !errors.As(err, &wrapped) {
		return false
	}
	for wrapped != nil {
		if wrapped.StatusCode != nil && *wrapped.StatusCode == http.StatusForbidden {
			return true
		}
		if wrapped.InnerError == nil {
			break
		}
		wrapped = wrapped.InnerError
	}
	return false
}
