package shared

import (
	"fmt"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

// ValidateEndpointAgainstMetadata validates that the given endpoint's Type and
// Authorization.Scheme and Parameters match the live metadata for the organization.
func ValidateEndpointAgainstMetadata(cmdCtx util.CmdContext, organization string, endpoint *serviceendpoint.ServiceEndpoint) error {
	if endpoint == nil {
		return fmt.Errorf("endpoint required")
	}
	if endpoint.Type == nil {
		return fmt.Errorf("endpoint.Type required")
	}

	types, err := GetServiceEndpointTypes(cmdCtx, organization)
	if err != nil {
		return err
	}

	var matched *serviceendpoint.ServiceEndpointType
	for _, t := range types {
		if t.Name != nil && *t.Name == *endpoint.Type {
			matched = &t
			break
		}
	}
	if matched == nil {
		return fmt.Errorf("unknown service endpoint type: %s", *endpoint.Type)
	}

	if endpoint.Authorization == nil || endpoint.Authorization.Scheme == nil {
		return fmt.Errorf("endpoint.Authorization.Scheme required for type validation")
	}

	scheme := *endpoint.Authorization.Scheme
	var matchedScheme *serviceendpoint.ServiceEndpointAuthenticationScheme
	for _, s := range *matched.AuthenticationSchemes {
		if s.Scheme != nil && *s.Scheme == scheme {
			matchedScheme = &s
			break
		}
	}
	if matchedScheme == nil {
		return fmt.Errorf("scheme %s not supported for type %s", scheme, *endpoint.Type)
	}

	// Validate required input descriptors
	params := map[string]string{}
	if endpoint.Authorization.Parameters != nil {
		params = *endpoint.Authorization.Parameters
	}

	if matchedScheme.InputDescriptors != nil {
		for _, desc := range *matchedScheme.InputDescriptors {
			if desc.Id == nil {
				continue
			}
			id := *desc.Id
			// If the input validation marks this as required, enforce presence.
			if desc.Validation != nil && desc.Validation.IsRequired != nil && *desc.Validation.IsRequired {
				if _, ok := params[id]; !ok {
					return fmt.Errorf("missing required auth parameter: %s", id)
				}
			}
		}
	}

	return nil
}
