package azdo

import (
	"context"
	"fmt"
	"strconv"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
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
