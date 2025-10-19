package shared

import (
	"fmt"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// Target encapsulates the parsed components of a security group target argument.
type Target struct {
	Organization string
	Project      string
	GroupName    string
}

// Scope describes the organization/project resolution for commands that accept optional project scope.
type Scope struct {
	Organization string
	Project      string
}

// ParseScope resolves the organization and optional project from an input argument of the form
// "[ORGANIZATION[/PROJECT]]". When the input is empty, the default organization from the user
// configuration is returned. The function trims whitespace around individual segments and ensures the
// resulting organization and project values are non-empty when provided.
func ParseScope(ctx util.CmdContext, scope string) (*Scope, error) {
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
			return nil, util.FlagErrorf("invalid scope format: %s", scope)
		}
		result.Organization = org
	case 2:
		org := strings.TrimSpace(parts[0])
		project := strings.TrimSpace(parts[1])
		if org == "" || project == "" {
			return nil, util.FlagErrorf("invalid scope format: %s", scope)
		}
		result.Organization = org
		result.Project = project
	default:
		return nil, util.FlagErrorf("invalid scope format: %s", scope)
	}

	return result, nil
}

// ParseTarget validates and parses a target argument of form ORGANIZATION/GROUP or ORGANIZATION/PROJECT/GROUP.
func ParseTarget(target string) (*Target, error) {
	if strings.TrimSpace(target) == "" {
		return nil, util.FlagErrorf("target must not be empty")
	}

	parts := strings.Split(target, "/")
	switch len(parts) {
	case 2:
		org := strings.TrimSpace(parts[0])
		groupName := strings.TrimSpace(parts[1])
		if org == "" || groupName == "" {
			return nil, util.FlagErrorf("invalid target format: %s", target)
		}
		return &Target{
			Organization: org,
			GroupName:    groupName,
		}, nil
	case 3:
		org := strings.TrimSpace(parts[0])
		project := strings.TrimSpace(parts[1])
		groupName := strings.TrimSpace(parts[2])
		if org == "" || project == "" || groupName == "" {
			return nil, util.FlagErrorf("invalid target format: %s", target)
		}
		return &Target{
			Organization: org,
			Project:      project,
			GroupName:    groupName,
		}, nil
	default:
		return nil, util.FlagErrorf("invalid target format: %s", target)
	}
}

// ParseTargetWithDefault resolves a target argument that allows an implicit organization by falling
// back to the configured default. The accepted formats are:
//   - "GROUP" (defaults organization, no project)
//   - "ORGANIZATION/GROUP"
//   - "ORGANIZATION/PROJECT/GROUP"
func ParseTargetWithDefault(ctx util.CmdContext, target string) (*Target, error) {
	if strings.TrimSpace(target) == "" {
		return nil, util.FlagErrorf("target must not be empty")
	}

	parts := strings.Split(target, "/")
	switch len(parts) {
	case 1:
		group := strings.TrimSpace(parts[0])
		if group == "" {
			return nil, util.FlagErrorf("invalid target format: %s", target)
		}
		scope, err := ParseScope(ctx, "")
		if err != nil {
			return nil, err
		}
		return &Target{
			Organization: scope.Organization,
			GroupName:    group,
		}, nil
	case 2:
		org := strings.TrimSpace(parts[0])
		group := strings.TrimSpace(parts[1])
		if org == "" || group == "" {
			return nil, util.FlagErrorf("invalid target format: %s", target)
		}
		return &Target{
			Organization: org,
			GroupName:    group,
		}, nil
	case 3:
		org := strings.TrimSpace(parts[0])
		project := strings.TrimSpace(parts[1])
		group := strings.TrimSpace(parts[2])
		if org == "" || project == "" || group == "" {
			return nil, util.FlagErrorf("invalid target format: %s", target)
		}
		return &Target{
			Organization: org,
			Project:      project,
			GroupName:    group,
		}, nil
	default:
		return nil, util.FlagErrorf("invalid target format: %s", target)
	}
}

// ResolveScopeDescriptor fetches the descriptor representing the project scope when a project is supplied.
// It returns the descriptor value along with the project ID string to support callers that need to distinguish
// between identically named groups scoped to different projects.
func ResolveScopeDescriptor(ctx util.CmdContext, organization, project string) (*string, *string, error) {
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

// FindGroupByName locates a single Azure DevOps security group by its display name and optional descriptor.
// When descriptorFilter is empty, exactly one group must match the provided name within the optional scope descriptor.
func FindGroupByName(ctx util.CmdContext, organization, project, groupName, descriptorFilter string) (*graph.GraphGroup, error) {
	scopeDescriptor, _, err := ResolveScopeDescriptor(ctx, organization, project)
	if err != nil {
		return nil, err
	}

	extensionsClient, err := ctx.ClientFactory().Extensions(ctx.Context(), organization)
	if err != nil {
		return nil, fmt.Errorf("failed to create extensions client: %w", err)
	}

	groups, err := extensionsClient.FindGroupsByDisplayName(ctx.Context(), groupName, scopeDescriptor)
	if err != nil {
		return nil, fmt.Errorf("failed to find group: %w", err)
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("no security group found with name %q", groupName)
	}

	if descriptorFilter != "" {
		for _, g := range groups {
			if types.GetValue(g.Descriptor, "") == descriptorFilter {
				return g, nil
			}
		}
		return nil, fmt.Errorf("no group found with the specified descriptor filter %q", descriptorFilter)
	}

	if len(groups) > 1 {
		return nil, fmt.Errorf("multiple groups found with name %q. specify a descriptor filter", groupName)
	}

	return groups[0], nil
}
