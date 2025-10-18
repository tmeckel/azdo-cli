package shared

import (
	"context"
	"fmt"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// GetReviewerIdentities resolves a list of reviewer handles (e.g., emails) into a map of handle-to-identity objects.
func GetReviewerIdentities(ctx context.Context, identityClient identity.Client, reviewerHandles []string) (map[string]identity.Identity, error) {
	if len(reviewerHandles) == 0 {
		return make(map[string]identity.Identity), nil
	}

	identitiesResult := make(map[string]identity.Identity, len(reviewerHandles))
	searchFilter := "MailAddress"
	for _, handle := range reviewerHandles {
		identities, err := identityClient.ReadIdentities(ctx, identity.ReadIdentitiesArgs{
			SearchFilter: &searchFilter,
			FilterValue:  &handle,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get identity for reviewer '%s': %w", handle, err)
		}
		if identities == nil || len(*identities) == 0 {
			return nil, fmt.Errorf("no identity found for reviewer '%s'", handle)
		}
		if len(*identities) > 1 {
			return nil, fmt.Errorf("multiple identities found for reviewer '%s', please use a more specific identifier", handle)
		}

		identitiesResult[handle] = (*identities)[0]
	}

	return identitiesResult, nil
}

// ResolveReviewers takes lists of required and optional reviewer handles, resolves them, and returns a de-duplicated list
// of IdentityRefWithVote objects. If a reviewer is in both lists, they are marked as required.
func ResolveReviewers(ctx context.Context, identityClient identity.Client, requiredReviewerHandles []string, optionalReviewerHandles []string) ([]git.IdentityRefWithVote, error) {
	handleRequiredMap := make(map[string]bool)
	for _, h := range optionalReviewerHandles {
		handleRequiredMap[h] = false
	}
	for _, h := range requiredReviewerHandles {
		handleRequiredMap[h] = true
	}

	if len(handleRequiredMap) == 0 {
		return []git.IdentityRefWithVote{}, nil
	}

	uniqueHandles := make([]string, 0, len(handleRequiredMap))
	for h := range handleRequiredMap {
		uniqueHandles = append(uniqueHandles, h)
	}

	resolvedIdentitiesMap, err := GetReviewerIdentities(ctx, identityClient, uniqueHandles)
	if err != nil {
		return nil, err
	}

	reviewersList := make([]git.IdentityRefWithVote, 0, len(resolvedIdentitiesMap))
	for handle, identity := range resolvedIdentitiesMap {
		isRequired := handleRequiredMap[handle]
		reviewersList = append(reviewersList, git.IdentityRefWithVote{
			Id:         types.ToPtr(identity.Id.String()),
			IsRequired: types.ToPtr(isRequired),
		})
	}
	return reviewersList, nil
}
