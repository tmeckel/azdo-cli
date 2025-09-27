package azdo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
)

type clientAdapter struct {
	instance *azuredevops.Client
}

var _ Client = (*clientAdapter)(nil)

func (c *clientAdapter) CreateRequestMessage(ctx context.Context, httpMethod string, url string, apiVersion string, body io.Reader, mediaType string, acceptMediaType string, additionalHeaders map[string]string) (*http.Request, error) {
	return c.instance.CreateRequestMessage(ctx, httpMethod, url, apiVersion, body, mediaType, acceptMediaType, additionalHeaders)
}

func (c *clientAdapter) GenerateUrl(apiResourceLocation *azuredevops.ApiResourceLocation, routeValues map[string]string, queryParameters url.Values) string {
	return c.instance.GenerateUrl(apiResourceLocation, routeValues, queryParameters)
}

func (c *clientAdapter) GetResourceAreas(ctx context.Context) (*[]azuredevops.ResourceAreaInfo, error) {
	return c.instance.GetResourceAreas(ctx)
}

func (c *clientAdapter) Send(ctx context.Context, httpMethod string, locationId uuid.UUID, apiVersion string, routeValues map[string]string, queryParameters url.Values, body io.Reader, mediaType string, acceptMediaType string, additionalHeaders map[string]string) (*http.Response, error) {
	return c.instance.Send(ctx, httpMethod, locationId, apiVersion, routeValues, queryParameters, body, mediaType, acceptMediaType, additionalHeaders)
}

func (c *clientAdapter) SendRequest(request *http.Request) (*http.Response, error) {
	return c.instance.SendRequest(request)
}

func (c *clientAdapter) UnmarshalBody(response *http.Response, v any) error {
	return c.instance.UnmarshalBody(response, v)
}

func (c *clientAdapter) UnmarshalCollectionBody(response *http.Response, v any) error {
	return c.instance.UnmarshalCollectionBody(response, v)
}

func (c *clientAdapter) UnmarshalCollectionJson(jsonValue []byte, v any) error {
	return c.instance.UnmarshalCollectionJson(jsonValue, v)
}

func (c *clientAdapter) UnwrapError(response *http.Response) error {
	return c.instance.UnwrapError(response)
}

type connectionAdapter struct {
	conn *azuredevops.Connection
}

var _ Connection = (*connectionAdapter)(nil)

func NewPatConnection(organizationUrl string, personalAccessToken string) Connection {
	return &connectionAdapter{
		conn: azuredevops.NewPatConnection(organizationUrl, personalAccessToken),
	}
}

func (c *connectionAdapter) GetClientByResourceAreaId(ctx context.Context, resourceAreaID uuid.UUID) (Client, error) {
	client, err := c.conn.GetClientByResourceAreaId(ctx, resourceAreaID)
	if err != nil {
		return nil, err
	}
	return &clientAdapter{
		instance: client,
	}, nil
}

func (c *connectionAdapter) GetClientByUrl(baseUrl string) Client {
	return &clientAdapter{
		instance: c.conn.GetClientByUrl(baseUrl),
	}
}

var rxOrgURL = regexp.MustCompile(`//(dev\.azure\.com/(?P<organization>[^/]+)|(?P<organization>[^.]+)\.visualstudio\.com)`)

func (c *connectionAdapter) Organization() string {
	match := rxOrgURL.FindStringSubmatch(c.conn.BaseUrl)
	if len(match) == 0 {
		panic(fmt.Errorf("invalid Azure DevOps URL %s", c.conn.BaseUrl))
	}
	return match[2] + match[3]
}
