package shared

import (
    "fmt"
    "sync"

    "github.com/tmeckel/azdo-cli/internal/cmd/util"
    "github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
)

var typesCache sync.Map // map[string][]serviceendpoint.ServiceEndpointType

// GetServiceEndpointTypes fetches service endpoint types for an organization and caches them for
// the duration of the command process. It uses the vendored Azure DevOps SDK.
func GetServiceEndpointTypes(cmdCtx util.CmdContext, organization string) ([]serviceendpoint.ServiceEndpointType, error) {
    if organization == "" {
        return nil, fmt.Errorf("organization required")
    }
    if v, ok := typesCache.Load(organization); ok {
        return v.([]serviceendpoint.ServiceEndpointType), nil
    }

    client, err := cmdCtx.ClientFactory().ServiceEndpoint(cmdCtx.Context(), organization)
    if err != nil {
        return nil, fmt.Errorf("create serviceendpoint client: %w", err)
    }

    res, err := client.GetServiceEndpointTypes(cmdCtx.Context(), serviceendpoint.GetServiceEndpointTypesArgs{})
    if err != nil {
        return nil, fmt.Errorf("get service endpoint types: %w", err)
    }
    if res == nil {
        return nil, fmt.Errorf("no service endpoint types returned")
    }

    typesCache.Store(organization, *res)
    return *res, nil
}

// Helpers for tests
func setTypesCacheForTest(org string, types []serviceendpoint.ServiceEndpointType) {
    typesCache.Store(org, types)
}

func clearTypesCacheForTest() {
    typesCache = sync.Map{}
}

