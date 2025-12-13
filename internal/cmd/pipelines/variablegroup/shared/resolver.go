package shared

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// ResolveVariableGroup retrieves a variable group by numeric ID or case-insensitive name and returns the SDK model.
func ResolveVariableGroup(
	cmdCtx util.CmdContext,
	client taskagent.Client,
	project string,
	target string,
) (*taskagent.VariableGroup, error) {
	if strings.TrimSpace(project) == "" {
		return nil, fmt.Errorf("project is required to resolve variable groups")
	}
	if strings.TrimSpace(target) == "" {
		return nil, util.FlagErrorf("variable group target cannot be empty")
	}

	if id, err := strconv.Atoi(target); err == nil {
		if id < 0 {
			return nil, util.FlagErrorf("invalid variable group id %d", id)
		}
		groups, err := client.GetVariableGroupsById(cmdCtx.Context(), taskagent.GetVariableGroupsByIdArgs{
			Project:  types.ToPtr(project),
			GroupIds: &[]int{id},
		})
		if err != nil {
			return nil, err
		}
		if groups == nil || len(*groups) == 0 {
			return nil, fmt.Errorf("variable group %q not found", target)
		}
		if (*groups)[0].Id == nil {
			return nil, fmt.Errorf("variable group %q returned without an ID", target)
		}
		return &(*groups)[0], nil
	}

	groups, err := client.GetVariableGroups(cmdCtx.Context(), taskagent.GetVariableGroupsArgs{
		Project:   types.ToPtr(project),
		GroupName: types.ToPtr(target),
	})
	if err != nil {
		return nil, err
	}
	var matches []*taskagent.VariableGroup
	if groups != nil {
		for i := range *groups {
			vg := &(*groups)[i]
			if vg.Name == nil {
				continue
			}
			if strings.EqualFold(*vg.Name, target) {
				matches = append(matches, vg)
			}
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("variable group %q not found", target)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple variable groups named %q found; specify the numeric ID", target)
	}
	if matches[0].Id == nil {
		return nil, fmt.Errorf("variable group %q returned without an ID", target)
	}
	return matches[0], nil
}
