package delete

import (
	"errors"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type deleteOptions struct {
	targetArg          string
	deep               bool
	yes                bool
	additionalProjects []string
}

type projectTarget struct {
	ID          string
	DisplayName string
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &deleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete [ORGANIZATION/]PROJECT/ID_OR_NAME",
		Short: "Delete a service endpoint from a project.",
		Long: heredoc.Doc(`
			Delete an Azure DevOps service endpoint (service connection) from a project.

			The positional argument accepts the form [ORGANIZATION/]PROJECT/ID_OR_NAME. When the
			organization segment is omitted the default organization from configuration is used.
		`),
		Example: heredoc.Doc(`
			# Delete by endpoint ID inside the default organization
			azdo service-endpoint delete MyProject/058bff6f-2717-4500-af7e-3fffc2b0b546

			# Delete by name inside a specific organization, removing shares in another project
			azdo service-endpoint delete myorg/MyProject/My Connection --additional-project myorg/SharedProject

			# Deep delete an AzureRM connection and suppress the confirmation
			azdo service-endpoint delete myorg/MyProject/ProdConnection --deep --yes
		`),
		Aliases: []string{
			"rm",
			"del",
			"d",
		},
		Args: util.ExactArgs(1, "service endpoint target required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runDelete(ctx, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.deep, "deep", false, "Also delete the backing Azure AD application for supported endpoints.")
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip the confirmation prompt.")
	cmd.Flags().StringArrayVar(&opts.additionalProjects, "additional-project", nil, "Additional project scope [ORGANIZATION/]PROJECT when the endpoint is shared. (Repeatable, comma-separated)")

	return cmd
}

func runDelete(ctx util.CmdContext, opts *deleteOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	prompter, err := ctx.Prompter()
	if err != nil {
		return err
	}

	scope, err := util.ParseProjectTargetWithDefaultOrganization(ctx, opts.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	var additionalScopes []*util.Scope
	if len(opts.additionalProjects) > 0 {
		additionalScopes = make([]*util.Scope, 0, len(opts.additionalProjects))
		for _, value := range opts.additionalProjects {
			additionalScope, parseErr := util.ParseProjectScope(ctx, value)
			if parseErr != nil {
				return util.FlagErrorWrap(parseErr)
			}
			if !strings.EqualFold(additionalScope.Organization, scope.Organization) {
				return util.FlagErrorf("additional project %q must belong to organization %s", value, scope.Organization)
			}
			additionalScopes = append(additionalScopes, additionalScope)
		}
	}

	if !opts.yes {
		message := fmt.Sprintf("Delete service endpoint %q from project %s/%s?", scope.Target, scope.Organization, scope.Project)
		var extra []string
		if opts.deep {
			extra = append(extra, "This will also delete the backing Azure AD application when supported.")
		}
		if len(additionalScopes) > 0 {
			extra = append(extra, fmt.Sprintf("This will also remove access from %d additional project(s).", len(additionalScopes)))
		}
		if len(extra) > 0 {
			message = fmt.Sprintf("%s\n%s", message, strings.Join(extra, " "))
		}

		confirmed, err := prompter.Confirm(message, false)
		if err != nil {
			return err
		}
		if !confirmed {
			return util.ErrCancel
		}
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	serviceEndpointClient, err := ctx.ClientFactory().ServiceEndpoint(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create service endpoint client: %w", err)
	}

	endpoint, err := shared.FindServiceEndpoint(ctx, serviceEndpointClient, scope.Project, scope.Target)
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

	coreClient, err := ctx.ClientFactory().Core(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create core client: %w", err)
	}

	projectTargets, err := buildProjectTargets(ctx, coreClient, scope, endpoint, additionalScopes)
	if err != nil {
		return err
	}

	projectIDs := make([]string, 0, len(projectTargets))
	for _, target := range projectTargets {
		projectIDs = append(projectIDs, target.ID)
	}

	zap.L().Debug("Deleting service endpoint",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("identifier", scope.Target),
		zap.Bool("deep", opts.deep),
		zap.Strings("projectIds", projectIDs),
	)

	deleteArgs := serviceendpoint.DeleteServiceEndpointArgs{
		EndpointId: endpoint.Id,
		ProjectIds: &projectIDs,
	}
	if opts.deep {
		deleteArgs.Deep = types.ToPtr(true)
	}

	if err := serviceEndpointClient.DeleteServiceEndpoint(ctx.Context(), deleteArgs); err != nil {
		return fmt.Errorf("failed to delete service endpoint %s: %w", endpoint.Id.String(), err)
	}

	ios.StopProgressIndicator()

	cs := ios.ColorScheme()
	name := strings.TrimSpace(types.GetValue(endpoint.Name, scope.Target))
	fmt.Fprintf(ios.Out, "%s Deleted service endpoint %q (%s) from %d project(s).\n", cs.SuccessIcon(), name, endpoint.Id.String(), len(projectTargets))

	return nil
}

func buildProjectTargets(ctx util.CmdContext, coreClient core.Client, primaryScope *util.Target, endpoint *serviceendpoint.ServiceEndpoint, additionalScopes []*util.Scope) ([]projectTarget, error) {
	idSet := make(map[string]struct{})
	var targets []projectTarget

	primaryDisplay := fmt.Sprintf("%s/%s", primaryScope.Organization, primaryScope.Project)
	primaryID := projectIDFromEndpoint(endpoint, primaryScope.Project)
	if primaryID == "" {
		resolvedID, err := resolveProjectID(ctx, coreClient, primaryScope.Project)
		if err != nil {
			return nil, err
		}
		primaryID = resolvedID
	}
	targets = appendProjectTarget(targets, idSet, projectTarget{ID: primaryID, DisplayName: primaryDisplay})

	for _, scope := range additionalScopes {
		resolvedID := projectIDFromEndpoint(endpoint, scope.Project)
		if resolvedID == "" {
			var err error
			resolvedID, err = resolveProjectID(ctx, coreClient, scope.Project)
			if err != nil {
				return nil, err
			}
		}
		display := fmt.Sprintf("%s/%s", scope.Organization, scope.Project)
		targets = appendProjectTarget(targets, idSet, projectTarget{ID: resolvedID, DisplayName: display})
	}

	return targets, nil
}

func appendProjectTarget(targets []projectTarget, idSet map[string]struct{}, target projectTarget) []projectTarget {
	if target.ID == "" {
		return targets
	}
	if _, exists := idSet[target.ID]; exists {
		return targets
	}
	idSet[target.ID] = struct{}{}
	return append(targets, target)
}

func resolveProjectID(ctx util.CmdContext, coreClient core.Client, project string) (string, error) {
	projectRef, err := coreClient.GetProject(ctx.Context(), core.GetProjectArgs{
		ProjectId: types.ToPtr(project),
	})
	if err != nil {
		return "", fmt.Errorf("failed to resolve project %q: %w", project, err)
	}
	if projectRef == nil || projectRef.Id == nil {
		return "", fmt.Errorf("project %q returned without an ID", project)
	}
	return projectRef.Id.String(), nil
}

func projectIDFromEndpoint(endpoint *serviceendpoint.ServiceEndpoint, projectName string) string {
	if endpoint == nil || endpoint.ServiceEndpointProjectReferences == nil {
		return ""
	}
	for _, ref := range *endpoint.ServiceEndpointProjectReferences {
		if ref.ProjectReference == nil || ref.ProjectReference.Id == nil || ref.ProjectReference.Name == nil {
			continue
		}
		if strings.EqualFold(*ref.ProjectReference.Name, projectName) {
			return ref.ProjectReference.Id.String()
		}
	}
	return ""
}
