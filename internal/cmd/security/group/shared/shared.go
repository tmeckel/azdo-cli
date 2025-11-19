package shared

import (
	"fmt"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

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
