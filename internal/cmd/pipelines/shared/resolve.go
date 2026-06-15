package shared

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// ResolvePipelineDefinition resolves a pipeline target by positive ID or definition name.
func ResolvePipelineDefinition(cmdCtx util.CmdContext, client build.Client, project, raw string) (int, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return 0, util.FlagErrorf("pipeline target cannot be empty")
	}
	if strings.TrimSpace(project) == "" {
		return 0, fmt.Errorf("project is required to resolve pipeline definitions")
	}

	if id, err := strconv.Atoi(target); err == nil {
		if id <= 0 {
			return 0, fmt.Errorf("pipeline id must be greater than zero: %q", target)
		}
		return id, nil
	}

	definitions, err := client.GetDefinitions(cmdCtx.Context(), build.GetDefinitionsArgs{
		Project: types.ToPtr(project),
		Name:    types.ToPtr(target),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to query pipeline definitions: %w", err)
	}
	if definitions == nil || len(definitions.Value) == 0 {
		return 0, fmt.Errorf("pipeline %q not found", target)
	}
	if len(definitions.Value) > 1 {
		return 0, fmt.Errorf("pipeline %q is ambiguous: %d matches found", target, len(definitions.Value))
	}

	id := types.GetValue(definitions.Value[0].Id, 0)
	if id <= 0 {
		return 0, fmt.Errorf("pipeline %q returned empty id", target)
	}
	return id, nil
}
