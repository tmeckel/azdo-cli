package shared

import (
	"errors"
	"fmt"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
)

// ValidateEndpointPayload validates the service endpoint payload.
//
// Baseline validation (always performed):
// - Trims whitespace from Name, Type, and Url if they are present.
// - Returns an error if Name, Type, or Url are present but empty (or whitespace only).
//
// Strict validation (requireIdentityFields = true):
// - Requires Name, Type, and Url to be present (non-nil).
func ValidateEndpointPayload(endpoint *serviceendpoint.ServiceEndpoint, requireIdentityFields bool) error {
	if endpoint == nil {
		return errors.New("service endpoint payload is nil")
	}

	// Baseline validation: normalize and check non-empty if present
	if endpoint.Name != nil {
		trimmed := strings.TrimSpace(*endpoint.Name)
		if trimmed == "" {
			return errors.New("field 'name' cannot be empty")
		}
		endpoint.Name = &trimmed
	}

	if endpoint.Type != nil {
		trimmed := strings.TrimSpace(*endpoint.Type)
		if trimmed == "" {
			return errors.New("field 'type' cannot be empty")
		}
		endpoint.Type = &trimmed
	}

	if endpoint.Url != nil {
		trimmed := strings.TrimSpace(*endpoint.Url)
		if trimmed == "" {
			return errors.New("field 'url' cannot be empty")
		}
		endpoint.Url = &trimmed
	}

	// Strict validation: require presence
	if requireIdentityFields {
		if endpoint.Name == nil {
			return fmt.Errorf("field 'name' is required")
		}
		if endpoint.Type == nil {
			return fmt.Errorf("field 'type' is required")
		}
		if endpoint.Url == nil {
			return fmt.Errorf("field 'url' is required")
		}
	}

	return nil
}
