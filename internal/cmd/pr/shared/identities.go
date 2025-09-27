package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/tmeckel/azdo-cli/internal/types"
)

func GetReviewerDescriptors(ctx context.Context, identityClient identity.Client, reviewerHandles []string) ([]string, error) {
	if len(reviewerHandles) == 0 {
		return []string{}, nil
	}
	emailsStr := strings.Join(reviewerHandles, ",")
	identities, err := identityClient.ReadIdentities(ctx, identity.ReadIdentitiesArgs{
		IdentityIds:  &emailsStr,
		SearchFilter: types.ToPtr("MailAddress"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get identities: %w", err)
	}
	if identities == nil {
		return nil, fmt.Errorf("no identities found")
	}
	if len(*identities) != len(reviewerHandles) {
		return nil, fmt.Errorf("could not find descriptors for all reviewers")
	}

	descriptors := make([]string, len(*identities))
	for i, identity := range *identities {
		if identity.Descriptor == nil {
			return nil, fmt.Errorf("identity descriptor is nil for reviewer %v", *identity.Id)
		}
		descriptors[i] = *identity.Descriptor
	}

	return descriptors, nil
}
