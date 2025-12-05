package test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/operations"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"

	"github.com/tmeckel/azdo-cli/internal/azdo"
	inttest "github.com/tmeckel/azdo-cli/internal/test"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type CtxKey string

const (
	CtxKeyProjectName CtxKey = "serviceendpoint/test/project-name"
	CtxKeyProjectID   CtxKey = "serviceendpoint/test/project-id"
	CtxKeyEndpointID  CtxKey = "serviceendpoint/test/endpoint-id"
)

type SharedProject struct {
	name        string
	id          string
	onceInit    sync.Once
	onceCleanup sync.Once
	ctx         inttest.TestContext
}

func NewSharedProject(name string) *SharedProject {
	return &SharedProject{name: name}
}

func (p *SharedProject) Ensure(ctx inttest.TestContext) error {
	var initErr error
	p.onceInit.Do(func() {
		projectID, err := ProvisionProject(ctx, p.name)
		if err != nil {
			initErr = err
			return
		}
		p.id = projectID
		p.ctx = ctx
	})
	if initErr != nil {
		return initErr
	}
	if strings.TrimSpace(p.id) == "" {
		return fmt.Errorf("shared project initialization incomplete")
	}
	ctx.SetValue(CtxKeyProjectName, p.name)
	ctx.SetValue(CtxKeyProjectID, p.id)
	return nil
}

func (p *SharedProject) Cleanup() error {
	var cleanupErr error
	p.onceCleanup.Do(func() {
		cleanupErr = DeleteProjectByID(p.ctx, p.id)
	})
	return cleanupErr
}

func ProvisionProject(ctx inttest.TestContext, projectName string) (string, error) {
	coreClient, err := ctx.ClientFactory().Core(ctx.Context(), ctx.Org())
	if err != nil {
		return "", fmt.Errorf("failed to create core client: %w", err)
	}

	teamProject := &core.TeamProject{
		Name: types.ToPtr(projectName),
	}
	vis := core.ProjectVisibilityValues.Private
	teamProject.Visibility = &vis

	processID, err := ResolveProcessTemplate(ctx, coreClient, "Agile")
	if err != nil {
		return "", err
	}

	teamProject.Capabilities = &map[string]map[string]string{
		"versioncontrol": {
			"sourceControlType": "Git",
		},
		"processTemplate": {
			"templateTypeId": processID,
		},
	}

	opRef, err := coreClient.QueueCreateProject(ctx.Context(), core.QueueCreateProjectArgs{
		ProjectToCreate: teamProject,
	})
	if err != nil {
		return "", fmt.Errorf("failed to queue project creation: %w", err)
	}

	if err := WaitForOperation(ctx, opRef); err != nil {
		return "", fmt.Errorf("project creation failed: %w", err)
	}

	project, err := coreClient.GetProject(ctx.Context(), core.GetProjectArgs{
		ProjectId: types.ToPtr(projectName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch created project %q: %w", projectName, err)
	}
	if project == nil || project.Id == nil {
		return "", fmt.Errorf("project %q returned empty id", projectName)
	}

	return project.Id.String(), nil
}

func DeleteProjectByID(ctx inttest.TestContext, projectID string) error {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil
	}
	context := context.Background()
	coreClient, err := ctx.ClientFactory().Core(context, ctx.Org())
	if err != nil {
		return fmt.Errorf("failed to create core client for cleanup: %w", err)
	}
	parsedProjectID, err := uuid.Parse(projectID)
	if err != nil {
		return fmt.Errorf("invalid project ID %q: %w", projectID, err)
	}
	op, err := coreClient.QueueDeleteProject(context, core.QueueDeleteProjectArgs{
		ProjectId: &parsedProjectID,
	})
	if err != nil {
		return fmt.Errorf("failed to queue project deletion: %w", err)
	}
	return WaitForOperation(ctx, op)
}

func ResolveProcessTemplate(ctx inttest.TestContext, client core.Client, preferred string) (string, error) {
	processes, err := client.GetProcesses(ctx.Context(), core.GetProcessesArgs{})
	if err != nil {
		return "", fmt.Errorf("failed to list processes: %w", err)
	}
	preferred = strings.TrimSpace(preferred)
	var fallback string
	for _, process := range *processes {
		if process.Id == nil {
			continue
		}
		if fallback == "" {
			fallback = process.Id.String()
		}
		if process.Name != nil && preferred != "" && strings.EqualFold(*process.Name, preferred) {
			return process.Id.String(), nil
		}
	}
	if fallback == "" {
		return "", fmt.Errorf("no processes available in organization %s", ctx.Org())
	}
	return fallback, nil
}

func WaitForOperation(ctx inttest.TestContext, opRef *operations.OperationReference) error {
	if opRef == nil {
		return fmt.Errorf("operation reference is nil")
	}
	context := context.Background()
	operationsClient, err := ctx.ClientFactory().Operations(context, ctx.Org())
	if err != nil {
		return fmt.Errorf("failed to create operations client: %w", err)
	}
	_, err = azdo.PollOperationResult(context, operationsClient, opRef, 10*time.Minute)
	return err
}

func CreateTestServiceEndpoint(ctx inttest.TestContext, projectName, projectID string) (string, string, error) {
	client, err := ctx.ClientFactory().ServiceEndpoint(ctx.Context(), ctx.Org())
	if err != nil {
		return "", "", fmt.Errorf("failed to create service endpoint client: %w", err)
	}
	projectUUID, err := uuid.Parse(projectID)
	if err != nil {
		return "", "", fmt.Errorf("invalid project id %q: %w", projectID, err)
	}

	endpointName := fmt.Sprintf("azdo-cli-acc-ep-%s", uuid.New().String())
	endpointType := "azurerm"
	endpointURL := "https://management.azure.com/"
	owner := "library"
	scheme := "ServicePrincipal"
	description := "Acceptance test endpoint"

	authParams := map[string]string{
		"tenantid":            uuid.New().String(),
		"serviceprincipalid":  uuid.New().String(),
		"authenticationType":  "spnKey",
		"serviceprincipalkey": "test-secret-value",
	}

	data := map[string]string{
		"environment":      "AzureCloud",
		"scopeLevel":       "Subscription",
		"subscriptionId":   uuid.New().String(),
		"subscriptionName": "Acceptance Test Subscription",
	}

	projectRef := &serviceendpoint.ProjectReference{
		Id:   &projectUUID,
		Name: &projectName,
	}

	endpoint := &serviceendpoint.ServiceEndpoint{
		Name:        &endpointName,
		Type:        &endpointType,
		Url:         &endpointURL,
		Description: &description,
		Owner:       &owner,
		Authorization: &serviceendpoint.EndpointAuthorization{
			Scheme:     &scheme,
			Parameters: &authParams,
		},
		Data: &data,
		ServiceEndpointProjectReferences: &[]serviceendpoint.ServiceEndpointProjectReference{
			{
				ProjectReference: projectRef,
				Name:             &endpointName,
				Description:      &description,
			},
		},
	}

	created, err := client.CreateServiceEndpoint(ctx.Context(), serviceendpoint.CreateServiceEndpointArgs{Endpoint: endpoint})
	if err != nil {
		return "", "", fmt.Errorf("failed to create test service endpoint: %w", err)
	}
	if created == nil || created.Id == nil {
		return "", "", fmt.Errorf("service endpoint create response missing id")
	}
	return created.Id.String(), endpointName, nil
}

func DeleteEndpointByID(ctx inttest.TestContext, endpointID, projectID string) error {
	endpointID = strings.TrimSpace(endpointID)
	projectID = strings.TrimSpace(projectID)
	if endpointID == "" || projectID == "" {
		return nil
	}
	client, err := ctx.ClientFactory().ServiceEndpoint(ctx.Context(), ctx.Org())
	if err != nil {
		return fmt.Errorf("failed to create service endpoint client: %w", err)
	}
	parsedEndpointID, err := uuid.Parse(endpointID)
	if err != nil {
		return fmt.Errorf("invalid endpoint ID %q: %w", endpointID, err)
	}
	projectIDs := []string{projectID}
	return client.DeleteServiceEndpoint(ctx.Context(), serviceendpoint.DeleteServiceEndpointArgs{
		EndpointId: &parsedEndpointID,
		ProjectIds: &projectIDs,
	})
}

func GetTestProjectName(ctx inttest.TestContext) (string, error) {
	if val, ok := ctx.Value(CtxKeyProjectName); ok {
		if name, _ := val.(string); strings.TrimSpace(name) != "" {
			return name, nil
		}
	}
	if name := strings.TrimSpace(ctx.Project()); name != "" {
		return name, nil
	}
	return "", fmt.Errorf("test project name not available")
}

func CleanupEndpointFromContext(ctx inttest.TestContext, keyEndpointID any, projectKey any) error {
	endpointVal, _ := ctx.Value(keyEndpointID)
	projectVal, _ := ctx.Value(projectKey)
	endpointID, _ := endpointVal.(string)
	projectID, _ := projectVal.(string)
	if strings.TrimSpace(endpointID) == "" || strings.TrimSpace(projectID) == "" {
		return nil
	}
	return DeleteEndpointByID(ctx, endpointID, projectID)
}
