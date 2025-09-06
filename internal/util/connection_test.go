package util_test

import (
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/util"
)

func TestOrganizationFromConnection(t *testing.T) {
	var conn *azuredevops.Connection

	for _, url := range []string{
		"https://dev.azure.com/org1",
		"https://org1.visualstudio.com",
		"https://org1.visualstudio.com/long/path/89?query=1",
		"https://dev.azure.com/org1/long/path/89?query=1",
	} {
		conn = &azuredevops.Connection{
			BaseUrl: url,
		}

		org, err := util.GetOrganizationFromConnection(conn)
		require.NoError(t, err)
		assert.Equal(t, "org1", org)
	}
}
