package shared

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

// ReadServiceEndpointFromFile reads, decodes, and parses a service endpoint from a file or stdin.
// It performs baseline validation (checks for empty fields if present) but does not require all fields.
func ReadServiceEndpointFromFile(stdin io.ReadCloser, path, encoding string) (*serviceendpoint.ServiceEndpoint, error) {
	encodingName, err := NormalizeEncoding(encoding)
	if err != nil {
		return nil, err
	}

	raw, err := util.ReadFile(path, stdin)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", DescribeInput(path), err)
	}

	decoded, err := DecodeContent(raw, encodingName)
	if err != nil {
		return nil, err
	}

	var endpoint serviceendpoint.ServiceEndpoint
	if err := json.Unmarshal(decoded, &endpoint); err != nil {
		return nil, fmt.Errorf("failed to parse JSON from %s: %w", DescribeInput(path), err)
	}

	if err := ValidateEndpointPayload(&endpoint, false); err != nil {
		return nil, err
	}

	return &endpoint, nil
}
