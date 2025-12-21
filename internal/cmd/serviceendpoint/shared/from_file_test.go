package shared

import (
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/types"
)

func TestReadServiceEndpointFromFile(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		encoding      string
		stdinContent  string
		fileContent   []byte
		createFile    bool
		expectedError string
		validate      func(t *testing.T, endpoint *serviceendpoint.ServiceEndpoint)
	}{
		{
			name:         "Valid from stdin",
			path:         "-",
			encoding:     "utf-8",
			stdinContent: `{"name": "stdin-ep", "type": "generic", "url": "http://localhost"}`,
			validate: func(t *testing.T, ep *serviceendpoint.ServiceEndpoint) {
				assert.Equal(t, "stdin-ep", types.GetValue(ep.Name, ""))
				assert.Equal(t, "generic", types.GetValue(ep.Type, ""))
				assert.Equal(t, "http://localhost", types.GetValue(ep.Url, ""))
			},
		},
		{
			name:        "Valid from file (UTF-8)",
			createFile:  true,
			encoding:    "utf-8",
			fileContent: []byte(`{"name": "file-ep", "type": "azurerm", "url": "https://management.azure.com/"}`),
			validate: func(t *testing.T, ep *serviceendpoint.ServiceEndpoint) {
				assert.Equal(t, "file-ep", types.GetValue(ep.Name, ""))
				assert.Equal(t, "azurerm", types.GetValue(ep.Type, ""))
				assert.Equal(t, "https://management.azure.com/", types.GetValue(ep.Url, ""))
			},
		},
		{
			name:          "Invalid encoding",
			path:          "-",
			encoding:      "bad-encoding",
			stdinContent:  `{}`,
			expectedError: `unsupported encoding "bad-encoding"`,
		},
		{
			name:          "File not found",
			path:          "non-existent-file.json",
			expectedError: "failed to read non-existent-file.json: open non-existent-file.json: no such file or directory",
		},
		{
			name:          "Bad JSON",
			path:          "-",
			stdinContent:  `{"name": "bad-json"`,
			expectedError: "failed to parse JSON from stdin: unexpected end of JSON input",
		},
		{
			name:          "Fails baseline validation (empty name)",
			path:          "-",
			stdinContent:  `{"name": "  ", "type": "generic", "url": "http://localhost"}`,
			expectedError: "field 'name' cannot be empty",
		},
		{
			name:          "Fails baseline validation (empty type)",
			path:          "-",
			stdinContent:  `{"name": "my-ep", "type": "", "url": "http://localhost"}`,
			expectedError: "field 'type' cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdin io.ReadCloser
			if tt.stdinContent != "" {
				stdin = io.NopCloser(strings.NewReader(tt.stdinContent))
			}

			path := tt.path
			if tt.createFile {
				tmpfile, err := ioutil.TempFile("", "test-*.json")
				require.NoError(t, err)
				defer os.Remove(tmpfile.Name())
				_, err = tmpfile.Write(tt.fileContent)
				require.NoError(t, err)
				require.NoError(t, tmpfile.Close())
				path = tmpfile.Name()
			}

			endpoint, err := ReadServiceEndpointFromFile(stdin, path, tt.encoding)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, endpoint)
			} else {
				require.NoError(t, err)
				require.NotNil(t, endpoint)
				if tt.validate != nil {
					tt.validate(t, endpoint)
				}
			}
		})
	}
}
