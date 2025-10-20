package util

import (
	"fmt"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// Scope describes the organization/project resolution for commands that accept optional project scope.
type Scope struct {
	Organization string
	Project      string
}

// ParseScope resolves the organization and optional project from an input argument of the form
// "[ORGANIZATION[/PROJECT]]". When the input is empty, the default organization from the user configuration
// is returned. The function trims whitespace around individual segments and ensures the resulting values are
// non-empty when provided.
func ParseScope(ctx CmdContext, scope string) (*Scope, error) {
	result := &Scope{}

	trimmed := strings.TrimSpace(scope)
	if trimmed == "" {
		cfg, err := ctx.Config()
		if err != nil {
			return nil, err
		}
		org, err := cfg.Authentication().GetDefaultOrganization()
		if err != nil {
			return nil, fmt.Errorf("no organization specified and no default organization configured: %w", err)
		}
		result.Organization = org
		return result, nil
	}

	parts := strings.Split(trimmed, "/")
	switch len(parts) {
	case 1:
		org := strings.TrimSpace(parts[0])
		if org == "" {
			return nil, FlagErrorf("invalid scope format: %s", scope)
		}
		result.Organization = org
	case 2:
		org := strings.TrimSpace(parts[0])
		project := strings.TrimSpace(parts[1])
		if org == "" || project == "" {
			return nil, FlagErrorf("invalid scope format: %s", scope)
		}
		result.Organization = org
		result.Project = project
	default:
		return nil, FlagErrorf("invalid scope format: %s", scope)
	}

	return result, nil
}

// ResolveScopeDescriptor fetches the descriptor representing the project scope when a project is supplied.
// It returns the descriptor value along with the project ID string to support callers that need to distinguish
// between identically named groups scoped to different projects.
func ResolveScopeDescriptor(ctx CmdContext, organization, project string) (*string, *string, error) {
	if project == "" {
		return nil, nil, nil
	}

	coreClient, err := ctx.ClientFactory().Core(ctx.Context(), organization)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create core client: %w", err)
	}

	projectRef, err := coreClient.GetProject(ctx.Context(), core.GetProjectArgs{
		ProjectId: types.ToPtr(project),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get project: %w", err)
	}
	if projectRef == nil || projectRef.Id == nil {
		return nil, nil, fmt.Errorf("project storage key is missing")
	}

	graphClient, err := ctx.ClientFactory().Graph(ctx.Context(), organization)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create graph client: %w", err)
	}

	descriptor, err := graphClient.GetDescriptor(ctx.Context(), graph.GetDescriptorArgs{
		StorageKey: projectRef.Id,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get project descriptor: %w", err)
	}
	if descriptor == nil || descriptor.Value == nil || *descriptor.Value == "" {
		return nil, nil, fmt.Errorf("project descriptor is empty")
	}

	var projectID *string
	if projectRef.Id != nil {
		id := projectRef.Id.String()
		projectID = &id
	}

	return descriptor.Value, projectID, nil
}
