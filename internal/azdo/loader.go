package azdo

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/tmeckel/azdo-cli/internal/azdo/extensions"
)

func GetProjects(ctx context.Context, client core.Client, args core.GetProjectsArgs) ([]core.TeamProjectReference, error) {
	var continuationToken *string

	result := []core.TeamProjectReference{}
	for {
		if continuationToken != nil {
			n, err := strconv.Atoi(*continuationToken)
			if err != nil {
				return []core.TeamProjectReference{}, fmt.Errorf("failed to parse continuation token %q to list projects: %w", *continuationToken, err)
			}
			args.ContinuationToken = &n
		}
		res, err := client.GetProjects(ctx, args)
		if err != nil {
			return []core.TeamProjectReference{}, err
		}

		if res == nil {
			break
		}
		if res.Value != nil {
			result = append(result, res.Value...)
		}
		if res.ContinuationToken == "" {
			break
		}

		continuationToken = &res.ContinuationToken
	}
	return result, nil
}

func GetVariableGroups(ctx context.Context, client extensions.Client, args taskagent.GetVariableGroupsArgs) ([]taskagent.VariableGroup, error) {
	project := ""
	if args.Project != nil {
		project = strings.TrimSpace(*args.Project)
	}
	if project == "" {
		return nil, fmt.Errorf("project is required to list variable groups")
	}

	var (
		result           []taskagent.VariableGroup
		nextTokenPointer = args.ContinuationToken
		nextTokenValue   int
	)

	for {
		currentArgs := args
		if nextTokenPointer != nil {
			currentArgs.ContinuationToken = nextTokenPointer
		} else {
			currentArgs.ContinuationToken = nil
		}

		resp, err := client.GetVariableGroups(ctx, currentArgs)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			break
		}

		if len(resp.Value) > 0 {
			result = append(result, resp.Value...)
		}

		if resp.ContinuationToken == nil || strings.TrimSpace(*resp.ContinuationToken) == "" {
			break
		}

		token, err := strconv.Atoi(strings.TrimSpace(*resp.ContinuationToken))
		if err != nil {
			return nil, fmt.Errorf("failed to parse continuation token %q to list variable groups: %w", *resp.ContinuationToken, err)
		}

		nextTokenValue = token
		nextTokenPointer = &nextTokenValue
	}

	return result, nil
}
