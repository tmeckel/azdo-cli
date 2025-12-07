package shared

import (
	"fmt"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// ResolveProjectReference fetches the project metadata required to attach service endpoints to a project.
// It returns a ProjectReference that includes the stable storage key (ID) and display name.
func ResolveProjectReference(ctx util.CmdContext, scope *util.Scope) (*serviceendpoint.ProjectReference, error) {
	if scope == nil {
		return nil, fmt.Errorf("scope is required")
	}
	if strings.TrimSpace(scope.Project) == "" {
		return nil, fmt.Errorf("project is required in scope")
	}

	coreClient, err := ctx.ClientFactory().Core(ctx.Context(), scope.Organization)
	if err != nil {
		return nil, fmt.Errorf("failed to create core client: %w", err)
	}

	project, err := coreClient.GetProject(ctx.Context(), core.GetProjectArgs{
		ProjectId: types.ToPtr(scope.Project),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project %q: %w", scope.Project, err)
	}
	if project == nil || project.Id == nil {
		return nil, fmt.Errorf("project %q does not expose an ID", scope.Project)
	}

	return &serviceendpoint.ProjectReference{
		Id:   project.Id,
		Name: project.Name,
	}, nil
}
