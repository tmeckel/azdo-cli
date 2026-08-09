package shared

import (
	"sync"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
)

// Helpers for tests

func setTypesCacheForTest(org string, types []serviceendpoint.ServiceEndpointType) {
	typesCache.Store(org, types)
}

func clearTypesCacheForTest() {
	typesCache = sync.Map{}
}
