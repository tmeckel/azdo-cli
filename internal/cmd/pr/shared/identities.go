package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/tmeckel/azdo-cli/internal/types"
)

func GetSelfID(ctx context.Context, conn *azuredevops.Connection) (uuid.UUID, error) {
	connectionDataUrl := fmt.Sprintf("%s/_apis/connectionData", conn.BaseUrl)
	connectionDataClient := conn.GetClientByUrl(connectionDataUrl)
	request, err := connectionDataClient.CreateRequestMessage(ctx, http.MethodGet, connectionDataUrl, "", nil, "", "", nil)
	if err != nil {
		return uuid.Nil, err
	}
	response, err := connectionDataClient.SendRequest(request)
	if err != nil {
		return uuid.Nil, err
	}
	defer response.Body.Close()

	var jsonData map[string]any
	err = json.NewDecoder(response.Body).Decode(&jsonData)
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.MustParse(((jsonData["authenticatedUser"].(map[string]any))["id"]).(string)), nil
}

func GetSubjectID(ctx context.Context, conn *azuredevops.Connection, subject string) (uuid.UUID, error) {
	graphClient, err := graph.NewClient(ctx, conn)
	if err != nil {
		return uuid.Nil, err
	}
	subjects, err := graphClient.QuerySubjects(ctx, graph.QuerySubjectsArgs{
		SubjectQuery: &graph.GraphSubjectQuery{
			Query: &subject,
			SubjectKind: &[]string{
				"User",
			},
		},
	})
	if err != nil {
		return uuid.Nil, err
	}
	if subjects == nil {
		return uuid.Nil, fmt.Errorf("user %s not found in organization", subject)
	} else if len(*subjects) != 1 {
		return uuid.Nil, fmt.Errorf("more than one user %s found in organization", subject)
	}
	storageKey, err := graphClient.GetStorageKey(ctx, graph.GetStorageKeyArgs{
		SubjectDescriptor: (*subjects)[0].Descriptor,
	})
	if err != nil {
		return uuid.Nil, err
	}
	if storageKey == nil {
		return uuid.Nil, fmt.Errorf("failed to get storage key for user %s (%s)", subject, *((*subjects)[0].Descriptor))
	}
	return *storageKey.Value, nil
}

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
