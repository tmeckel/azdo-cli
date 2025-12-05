package shared

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// ErrEndpointNotFound indicates that the requested service endpoint could not be located in the target project.
var ErrEndpointNotFound = errors.New("service endpoint not found")

// FindServiceEndpoint resolves a service endpoint by ID or name within a project. It returns the endpoint, a flag
// indicating whether the resolution used the name lookup path, or ErrEndpointNotFound when nothing matched.
func FindServiceEndpoint(ctx util.CmdContext, client serviceendpoint.Client, project, identifier string) (*serviceendpoint.ServiceEndpoint, error) {
	if parsedID, err := uuid.Parse(identifier); err == nil {
		endpoint, getErr := client.GetServiceEndpointDetails(ctx.Context(), serviceendpoint.GetServiceEndpointDetailsArgs{
			Project:    types.ToPtr(project),
			EndpointId: types.ToPtr(parsedID),
		})
		if getErr != nil {
			if util.IsNotFoundError(getErr) {
				return nil, ErrEndpointNotFound
			}
			return nil, fmt.Errorf("failed to get service endpoint %s: %w", parsedID.String(), getErr)
		}
		if endpoint.Id == nil { // GetServiceEndpointDetails returns an empty result instead of nil or an HTTP 404
			return nil, ErrEndpointNotFound
		}
		return endpoint, nil
	}

	nameSlice := []string{identifier}
	endpoints, err := client.GetServiceEndpointsByNames(ctx.Context(), serviceendpoint.GetServiceEndpointsByNamesArgs{
		Project:       types.ToPtr(project),
		EndpointNames: &nameSlice,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to lookup service endpoint %q: %w", identifier, err)
	}
	if endpoints == nil {
		return nil, ErrEndpointNotFound
	}
	for _, ep := range *endpoints {
		name := types.GetValue(ep.Name, "")
		if strings.EqualFold(name, identifier) {
			return &ep, nil
		}
	}
	return nil, ErrEndpointNotFound
}
