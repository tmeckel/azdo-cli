package shared

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

func ResolvePoolAgent(
	cmdCtx util.CmdContext,
	client taskagent.Client,
	org, poolTarget, agentTarget string,
) (*taskagent.TaskAgent, error) {
	if strings.TrimSpace(poolTarget) == "" {
		return nil, util.FlagErrorf("pool target cannot be empty")
	}
	if strings.TrimSpace(agentTarget) == "" {
		return nil, util.FlagErrorf("agent target cannot be empty")
	}

	poolID, err := ResolvePool(cmdCtx, client, poolTarget)
	if err != nil {
		return nil, err
	}

	agentID, err := ResolveAgent(cmdCtx, client, poolID, agentTarget)
	if err != nil {
		return nil, err
	}

	agent, err := client.GetAgent(cmdCtx.Context(), taskagent.GetAgentArgs{
		PoolId:  types.ToPtr(poolID),
		AgentId: types.ToPtr(agentID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}
	if agent == nil {
		return nil, fmt.Errorf("agent %q not found", agentTarget)
	}

	return agent, nil
}

func ResolvePool(cmdCtx util.CmdContext, client taskagent.Client, target string) (int, error) {
	if id, err := strconv.Atoi(target); err == nil {
		if id < 0 {
			return 0, util.FlagErrorf("invalid pool id %d", id)
		}
		return id, nil
	}

	pools, err := client.GetAgentPools(cmdCtx.Context(), taskagent.GetAgentPoolsArgs{
		PoolName: types.ToPtr(target),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list agent pools: %w", err)
	}

	var matches []taskagent.TaskAgentPool
	if pools != nil {
		for _, p := range *pools {
			if p.Name != nil && strings.EqualFold(*p.Name, target) {
				matches = append(matches, p)
			}
		}
	}

	if len(matches) == 0 {
		return 0, fmt.Errorf("pool %q not found", target)
	}
	if len(matches) > 1 {
		return 0, fmt.Errorf("multiple pools named %q found; specify the numeric ID", target)
	}
	if matches[0].Id == nil {
		return 0, fmt.Errorf("pool %q returned without an ID", target)
	}
	return *matches[0].Id, nil
}

func ResolveAgent(cmdCtx util.CmdContext, client taskagent.Client, poolID int, target string) (int, error) {
	if id, err := strconv.Atoi(target); err == nil {
		if id < 0 {
			return 0, util.FlagErrorf("invalid agent id %d", id)
		}
		return id, nil
	}

	agents, err := client.GetAgents(cmdCtx.Context(), taskagent.GetAgentsArgs{
		PoolId:    types.ToPtr(poolID),
		AgentName: types.ToPtr(target),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list agents in pool %d: %w", poolID, err)
	}

	var matches []taskagent.TaskAgent
	if agents != nil {
		for _, a := range *agents {
			if a.Name != nil && strings.EqualFold(*a.Name, target) {
				matches = append(matches, a)
			}
		}
	}

	if len(matches) == 0 {
		return 0, fmt.Errorf("agent %q not found in pool %d", target, poolID)
	}
	if len(matches) > 1 {
		return 0, fmt.Errorf("multiple agents named %q found in pool %d; specify the numeric ID", target, poolID)
	}
	if matches[0].Id == nil {
		return 0, fmt.Errorf("agent %q returned without an ID", target)
	}
	return *matches[0].Id, nil
}
