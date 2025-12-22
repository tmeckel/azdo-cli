package shared

import (
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tmeckel/azdo-cli/internal/types"
)

func TestValidateEndpointPayload(t *testing.T) {
	t.Parallel()

	t.Run("nil payload", func(t *testing.T) {
		t.Parallel()
		err := ValidateEndpointPayload(nil, false)
		require.EqualError(t, err, "service endpoint payload is nil")
	})

	t.Run("baseline: nil identity fields allowed when not strict", func(t *testing.T) {
		t.Parallel()
		endpoint := &serviceendpoint.ServiceEndpoint{}
		err := ValidateEndpointPayload(endpoint, false)
		require.NoError(t, err)
	})

	t.Run("strict: missing required identity fields", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name    string
			payload *serviceendpoint.ServiceEndpoint
			wantErr string
		}{
			{
				name:    "name missing",
				payload: &serviceendpoint.ServiceEndpoint{Type: types.ToPtr("t"), Url: types.ToPtr("u")},
				wantErr: "field 'name' is required",
			},
			{
				name:    "type missing",
				payload: &serviceendpoint.ServiceEndpoint{Name: types.ToPtr("n"), Url: types.ToPtr("u")},
				wantErr: "field 'type' is required",
			},
			{
				name:    "url missing",
				payload: &serviceendpoint.ServiceEndpoint{Name: types.ToPtr("n"), Type: types.ToPtr("t")},
				wantErr: "field 'url' is required",
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				err := ValidateEndpointPayload(tt.payload, true)
				require.EqualError(t, err, tt.wantErr)
			})
		}
	})

	t.Run("baseline: present identity fields cannot be empty or whitespace-only", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name    string
			payload *serviceendpoint.ServiceEndpoint
			wantErr string
		}{
			{
				name:    "name empty",
				payload: &serviceendpoint.ServiceEndpoint{Name: types.ToPtr("   "), Type: types.ToPtr("t"), Url: types.ToPtr("u")},
				wantErr: "field 'name' cannot be empty",
			},
			{
				name:    "type empty",
				payload: &serviceendpoint.ServiceEndpoint{Name: types.ToPtr("n"), Type: types.ToPtr("\t"), Url: types.ToPtr("u")},
				wantErr: "field 'type' cannot be empty",
			},
			{
				name:    "url empty",
				payload: &serviceendpoint.ServiceEndpoint{Name: types.ToPtr("n"), Type: types.ToPtr("t"), Url: types.ToPtr("\n")},
				wantErr: "field 'url' cannot be empty",
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				err := ValidateEndpointPayload(tt.payload, false)
				require.EqualError(t, err, tt.wantErr)
			})
		}
	})

	t.Run("trims identity fields when present", func(t *testing.T) {
		t.Parallel()
		endpoint := &serviceendpoint.ServiceEndpoint{
			Name: types.ToPtr("  example  "),
			Type: types.ToPtr("\tgeneric\t"),
			Url:  types.ToPtr("  http://localhost  "),
		}

		err := ValidateEndpointPayload(endpoint, true)
		require.NoError(t, err)

		require.NotNil(t, endpoint.Name)
		require.NotNil(t, endpoint.Type)
		require.NotNil(t, endpoint.Url)

		assert.Equal(t, "example", *endpoint.Name)
		assert.Equal(t, "generic", *endpoint.Type)
		assert.Equal(t, "http://localhost", *endpoint.Url)
	})

	t.Run("strict: whitespace-only identity fields fail baseline validation before strict checks", func(t *testing.T) {
		t.Parallel()
		endpoint := &serviceendpoint.ServiceEndpoint{
			Name: types.ToPtr(" "),
			Type: nil,
			Url:  nil,
		}

		err := ValidateEndpointPayload(endpoint, true)
		require.EqualError(t, err, "field 'name' cannot be empty")
	})
}
