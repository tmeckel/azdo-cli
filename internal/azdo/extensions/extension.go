package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// Client defines extension methods to the Azure DevOps REST API
type Client interface {
	// GetSelfID retrieves the storage identifier of the authenticated user associated with the connection.
	GetSelfID(ctx context.Context) (uuid.UUID, error)
	// GetSubjectID resolves the storage key (UUID) for a given subject (user) name within the organization.
	GetSubjectID(ctx context.Context, subject string) (uuid.UUID, error)
	// FindGroupsByDisplayName locates Azure DevOps security groups that match the provided display name,
	// optionally scoped to a project descriptor, and returns their full details.
	FindGroupsByDisplayName(ctx context.Context, displayName string, scopeDescriptor *string) ([]*graph.GraphGroup, error)
}

type extensionClient struct {
	conn *azuredevops.Connection
}

func NewClient(ctx context.Context, connection *azuredevops.Connection) Client {
	return &extensionClient{
		conn: connection,
	}
}

// GetSelfID retrieves the storage identifier of the authenticated user associated with the connection.
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

// GetSubjectID resolves the storage key (UUID) for a given subject (user) name within the organization.
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

// FindGroupsByDisplayName locates Azure DevOps security groups that match the provided display name,
// optionally scoped to a project descriptor, and returns their full details.
func (c *extensionClient) FindGroupsByDisplayName(ctx context.Context, displayName string, scopeDescriptor *string) ([]*graph.GraphGroup, error) {
	graphClient, err := graph.NewClient(ctx, c.conn)
	if err != nil {
		return nil, err
	}

	subjectKinds := []string{"Group"}
	query := graph.GraphSubjectQuery{
		Query:       types.ToPtr(displayName),
		SubjectKind: &subjectKinds,
	}
	if scopeDescriptor != nil && types.GetValue(scopeDescriptor, "") != "" {
		query.ScopeDescriptor = scopeDescriptor
	}

	subjects, err := graphClient.QuerySubjects(ctx, graph.QuerySubjectsArgs{
		SubjectQuery: &query,
	})
	if err != nil {
		return nil, err
	}
	if subjects == nil {
		return nil, nil
	}

	matched := make([]*graph.GraphGroup, 0)
	seen := make(map[string]struct{})

	for _, subject := range *subjects {
		descriptor := types.GetValue(subject.Descriptor, "")
		if descriptor == "" {
			continue
		}
		display := types.GetValue(subject.DisplayName, "")
		if !strings.EqualFold(display, displayName) {
			continue
		}
		if _, ok := seen[descriptor]; ok {
			continue
		}

		groupDescriptor := descriptor
		group, err := graphClient.GetGroup(ctx, graph.GetGroupArgs{
			GroupDescriptor: &groupDescriptor,
		})
		if err != nil {
			return nil, err
		}
		if group != nil {
			matched = append(matched, group)
			seen[descriptor] = struct{}{}
		}
	}

	return matched, nil
}
