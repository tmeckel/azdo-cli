package shared

import (
    "context"
    "errors"
    "fmt"
    "time"

    pollutil "github.com/tmeckel/azdo-cli/internal/util"
    "github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
)

// WaitForReady polls GetServiceEndpointDetails until IsReady==true or a terminal
// failed state is detected in OperationStatus, or context/timeout occurs.
func WaitForReady(ctx context.Context, client serviceendpoint.Client, project string, ep *serviceendpoint.ServiceEndpoint, timeout time.Duration) (*serviceendpoint.ServiceEndpoint, error) {
    if ep == nil || ep.Id == nil {
        return nil, fmt.Errorf("endpoint or id missing")
    }
    opts := pollutil.PollOptions{Tries: 0}
    if timeout > 0 {
        opts.Timeout = timeout
    }

    var last *serviceendpoint.ServiceEndpoint
    err := pollutil.Poll(ctx, func() error {
        id := *ep.Id
        res, err := client.GetServiceEndpointDetails(ctx, serviceendpoint.GetServiceEndpointDetailsArgs{
            Project:    &project,
            EndpointId: &id,
        })
        if err != nil {
            // transient error, retry
            last = nil
            return err
        }
        last = res
        if res != nil && res.IsReady != nil && *res.IsReady {
            return nil
        }
        // Inspect OperationStatus defensively for a failure signal
        if res != nil && res.OperationStatus != nil {
            if opMap, ok := res.OperationStatus.(map[string]any); ok {
                if stateRaw, ok := opMap["state"]; ok {
                    if stateStr, ok := stateRaw.(string); ok {
                        if stateStr == "failed" {
                            return fmt.Errorf("service endpoint creation failed: %v", res.OperationStatus)
                        }
                    }
                }
            }
        }
        return errors.New("not ready")
    }, opts)

    if err != nil {
        return last, err
    }
    return last, nil
}
