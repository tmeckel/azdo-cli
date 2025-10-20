package shared

import (
	"fmt"
	"strings"

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
		scope, err := util.ParseScope(ctx, "")
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

// FindGroupByName locates a single Azure DevOps security group by its display name and optional descriptor.
// When descriptorFilter is empty, exactly one group must match the provided name within the optional scope descriptor.
func FindGroupByName(ctx util.CmdContext, organization, project, groupName, descriptorFilter string) (*graph.GraphGroup, error) {
	scopeDescriptor, _, err := util.ResolveScopeDescriptor(ctx, organization, project)
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
