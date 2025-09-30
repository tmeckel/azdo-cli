package azdo

import (
	"context"
	"fmt"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/operations"
)

const (
	defaultPollInterval = 1 * time.Second
	defaultPollTimeout  = 1 * time.Minute
)

// PollOperationResult polls the status of a long-running operation until it completes or times out.
func PollOperationResult(ctx context.Context, client operations.Client, opRef *operations.OperationReference, timeout time.Duration) (*operations.Operation, error) {
	return PollOperationResultWithState(ctx, client, opRef, timeout, operations.OperationStatusValues.Succeeded)
}

// PollOperationResultWithState polls the status of a long-running operation until it reaches the target state or times out.
func PollOperationResultWithState(ctx context.Context, client operations.Client, opRef *operations.OperationReference, timeout time.Duration, targetState operations.OperationStatus) (*operations.Operation, error) {
	if timeout == 0 {
		timeout = defaultPollTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(defaultPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for operation %s to complete: %w", *opRef.Id, ctx.Err())
		case <-ticker.C:
			op, err := client.GetOperation(ctx, operations.GetOperationArgs{
				OperationId: opRef.Id,
				PluginId:    opRef.PluginId,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to get operation status for %s: %w", *opRef.Id, err)
			}

			if *op.Status == targetState {
				return op, nil
			}

			switch *op.Status {
			case operations.OperationStatusValues.Failed, operations.OperationStatusValues.Cancelled:
				detailedMessage := "no detailed message"
				if op.DetailedMessage != nil {
					detailedMessage = *op.DetailedMessage
				}
				return op, fmt.Errorf("operation %s did not succeed: status=%s, message=%s", *op.Id, *op.Status, detailedMessage)
			}
		}
	}
}
