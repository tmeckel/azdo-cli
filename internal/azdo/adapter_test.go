package azdo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tmeckel/azdo-cli/internal/azdo"
)

func TestOrganizationFromConnection(t *testing.T) {
	for _, url := range []string{
		"https://dev.azure.com/org1",
		"https://org1.visualstudio.com",
		"https://org1.visualstudio.com/long/path/89?query=1",
		"https://dev.azure.com/org1/long/path/89?query=1",
	} {
		conn := azdo.NewPatConnection(url, "pat")

		org := conn.Organization()
		assert.Equal(t, "org1", org)
	}
}
