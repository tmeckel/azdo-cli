package shared

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// ErrEndpointNotFound indicates that the requested service endpoint could not be located in the target project.
var ErrEndpointNotFound = errors.New("service endpoint not found")

// FindServiceEndpoint resolves a service endpoint by ID or name within a project. It returns the endpoint, a flag
// indicating whether the resolution used the name lookup path, or ErrEndpointNotFound when nothing matched.
func FindServiceEndpoint(ctx util.CmdContext, client serviceendpoint.Client, project, identifier string) (*serviceendpoint.ServiceEndpoint, error) {
	logger := zap.L().With(
		zap.String("project", project),
		zap.String("identifier", identifier),
	)

	if parsedID, err := uuid.Parse(identifier); err == nil {
		logger.Debug("resolving service endpoint by ID")
		endpoint, getErr := client.GetServiceEndpointDetails(ctx.Context(), serviceendpoint.GetServiceEndpointDetailsArgs{
			Project:    types.ToPtr(project),
			EndpointId: types.ToPtr(parsedID),
		})
		if getErr != nil {
			logger.Debug("service endpoint lookup by ID failed", zap.Error(getErr))
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

	logger.Debug("resolving service endpoint by name")
	nameSlice := []string{identifier}
	endpoints, err := client.GetServiceEndpointsByNames(ctx.Context(), serviceendpoint.GetServiceEndpointsByNamesArgs{
		Project:       types.ToPtr(project),
		EndpointNames: &nameSlice,
		IncludeFailed: types.ToPtr(true),
	})
	if err != nil {
		logger.Debug("service endpoint lookup by name failed", zap.Error(err))
		return nil, fmt.Errorf("failed to lookup service endpoint %q: %w", identifier, err)
	}
	if endpoints == nil {
		logger.Debug("service endpoint lookup by name returned no results")
		return nil, ErrEndpointNotFound
	}
	for _, ep := range *endpoints {
		name := types.GetValue(ep.Name, "")
		if strings.EqualFold(name, identifier) {
			logger.Debug("resolved service endpoint by name", zap.String("matchedName", name))
			return &ep, nil
		}
		logger.Debug("service endpoing with name does not match", zap.String("name", name))
	}
	logger.Debug("no service endpoint matched provided name")
	return nil, ErrEndpointNotFound
}

// AuthorizationScheme returns the authorization scheme of the service endpoint or an empty string if not available.
func AuthorizationScheme(ep *serviceendpoint.ServiceEndpoint) string {
	if ep != nil && ep.Authorization != nil && ep.Authorization.Scheme != nil {
		return *ep.Authorization.Scheme
	}
	return ""
}

// RedactSecrets masks sensitive authorization parameters in the service endpoint.
func RedactSecrets(ep *serviceendpoint.ServiceEndpoint) {
	if ep == nil || ep.Authorization == nil || ep.Authorization.Parameters == nil {
		return
	}
	for k := range *ep.Authorization.Parameters {
		(*ep.Authorization.Parameters)[k] = "REDACTED"
	}
}
