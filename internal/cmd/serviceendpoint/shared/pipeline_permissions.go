package shared

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/pipelinepermissions"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

const (
	// EndpointResourceType is the Azure DevOps pipeline permission resource type for service connections.
	EndpointResourceType = "endpoint"
)

// CleanupFunc allows callers to provide optional rollback logic when granting permissions fails.
type CleanupFunc func() error

// GrantAllPipelinesAccessToEndpoint allows every pipeline in the specified project to use the service endpoint.
func GrantAllPipelinesAccessToEndpoint(
	cmdCtx util.CmdContext,
	organization string,
	projectID uuid.UUID,
	endpointID uuid.UUID,
	cleanup CleanupFunc,
) error {
	if cmdCtx == nil {
		return errors.New("nil command context")
	}
	if organization == "" {
		return errors.New("organization is required")
	}
	if projectID == uuid.Nil {
		return errors.New("project ID is required")
	}
	if endpointID == uuid.Nil {
		return errors.New("endpoint ID is required")
	}

	permissionsClient, err := cmdCtx.ClientFactory().PipelinePermissions(cmdCtx.Context(), organization)
	if err != nil {
		return runCleanup(fmt.Errorf("failed to initialize pipeline permissions client: %w", err), cleanup)
	}

	allPipelines := true
	projectIDStr := projectID.String()
	resourceType := EndpointResourceType
	resourceID := endpointID.String()

	_, err = permissionsClient.UpdatePipelinePermisionsForResource(cmdCtx.Context(), pipelinepermissions.UpdatePipelinePermisionsForResourceArgs{
		Project:      &projectIDStr,
		ResourceType: &resourceType,
		ResourceId:   &resourceID,
		ResourceAuthorization: &pipelinepermissions.ResourcePipelinePermissions{
			AllPipelines: &pipelinepermissions.Permission{
				Authorized: &allPipelines,
			},
		},
	})
	if err != nil {
		return runCleanup(fmt.Errorf("failed to authorize endpoint %s for all pipelines: %w", endpointID, err), cleanup)
	}

	return nil
}

func runCleanup(opErr error, cleanup CleanupFunc) error {
	if cleanup == nil {
		return opErr
	}
	if err := cleanup(); err != nil {
		return fmt.Errorf("%w (cleanup failed: %v)", opErr, err)
	}
	return opErr
}
