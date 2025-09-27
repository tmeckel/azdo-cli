package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
)

// Client defines extension methods to the Azure DevOps REST API
type Client interface {
	GetSelfID(ctx context.Context) (uuid.UUID, error)
	GetSubjectID(ctx context.Context, subject string) (uuid.UUID, error)
}

type extensionClient struct {
	conn *azuredevops.Connection
}

func NewClient(ctx context.Context, connection *azuredevops.Connection) Client {
	return &extensionClient{
		conn: connection,
	}
}

func (c *extensionClient) GetSelfID(ctx context.Context) (uuid.UUID, error) {
	connectionDataUrl := fmt.Sprintf("%s/_apis/connectionData", c.conn.BaseUrl)
	connectionDataClient := c.conn.GetClientByUrl(connectionDataUrl)
	request, err := connectionDataClient.CreateRequestMessage(ctx, http.MethodGet, connectionDataUrl, "", nil, "", "", nil)
	if err != nil {
		return uuid.Nil, err
	}
	response, err := connectionDataClient.SendRequest(request)
	if err != nil {
		return uuid.Nil, err
	}
	defer response.Body.Close()

	var payload struct {
		AuthenticatedUser struct {
			Id string `json:"id"`
		} `json:"authenticatedUser"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return uuid.Nil, err
	}
	if payload.AuthenticatedUser.Id == "" {
		return uuid.Nil, fmt.Errorf("authenticated user id not found")
	}
	return uuid.MustParse(payload.AuthenticatedUser.Id), nil
}

func (c *extensionClient) GetSubjectID(ctx context.Context, subject string) (uuid.UUID, error) {
	graphClient, err := graph.NewClient(ctx, c.conn)
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
