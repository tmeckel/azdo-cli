package azdo

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	azdogit "github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/operations"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/tmeckel/azdo-cli/internal/azdo/extensions"
)

type Client interface {
	CreateRequestMessage(ctx context.Context, httpMethod string, url string, apiVersion string, body io.Reader, mediaType string, acceptMediaType string, additionalHeaders map[string]string) (*http.Request, error)
	GenerateUrl(apiResourceLocation *azuredevops.ApiResourceLocation, routeValues map[string]string, queryParameters url.Values) string
	GetResourceAreas(ctx context.Context) (*[]azuredevops.ResourceAreaInfo, error)
	Send(ctx context.Context, httpMethod string, locationId uuid.UUID, apiVersion string, routeValues map[string]string, queryParameters url.Values, body io.Reader, mediaType string, acceptMediaType string, additionalHeaders map[string]string) (*http.Response, error)
	SendRequest(request *http.Request) (*http.Response, error)
	UnmarshalBody(response *http.Response, v any) error
	UnmarshalCollectionBody(response *http.Response, v any) error
	UnmarshalCollectionJson(jsonValue []byte, v any) error
	UnwrapError(response *http.Response) error
}

// Connection mirrors the exported methods on azuredevops.Connection for creating service clients.
// It intentionally returns the local Client interface to keep code decoupled from the vendor type.
type Connection interface {
	GetClientByResourceAreaId(ctx context.Context, resourceAreaID uuid.UUID) (Client, error)
	GetClientByUrl(baseUrl string) Client
	Organization() string
}

// ConnectionFactory provides connections and org-scoped SDK clients.
type ConnectionFactory interface {
	Connection(organization string) (Connection, error)
}

type ClientFactory interface {
	Git(ctx context.Context, organization string) (azdogit.Client, error)
	Identity(ctx context.Context, organization string) (identity.Client, error)
	Graph(ctx context.Context, organization string) (graph.Client, error)
	Core(ctx context.Context, organization string) (core.Client, error)
	Operations(ctx context.Context, organization string) (operations.Client, error)
	ServiceEndpoint(ctx context.Context, organization string) (serviceendpoint.Client, error)
	Security(ctx context.Context, organization string) (security.Client, error)
	Extensions(ctx context.Context, organization string) (extensions.Client, error)
	WorkItemTracking(ctx context.Context, organization string) (workitemtracking.Client, error)
}
