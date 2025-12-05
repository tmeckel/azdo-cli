package azurerm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/operations"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"

	"github.com/tmeckel/azdo-cli/internal/azdo"
	inttest "github.com/tmeckel/azdo-cli/internal/test"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type contextKey string

const (
	ctxKeyCreateOpts        contextKey = "azurerm/create-opts"
	ctxKeyEndpointID        contextKey = "azurerm/endpoint-id"
	ctxKeyEndpointProjectID contextKey = "azurerm/project-id"
	ctxKeyCertPath          contextKey = "azurerm/cert-path"
	ctxKeyProjectName       contextKey = "azurerm/test-project-name"
	ctxKeyProjectID         contextKey = "azurerm/test-project-id"
	testCertificatePEM                 = `-----BEGIN CERTIFICATE-----
MIIDXTCCAkWgAwIBAgIJAKoK/heBjcOuMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwHhcNMTIwOTEyMjE1MjAyWhcNMTUwOTEyMjE1MjAyWjBF
MQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50
ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBC
gKCAQEAwuTanj/Uo5Yhq7ckmL5jycB3Z/zPBuZjviQ4fAar/7xeOUe7/y2Kpls=
-----END CERTIFICATE-----`
)

func TestAccCreateAzureRMServiceEndpoint(t *testing.T) {
	sharedProj := newProject(fmt.Sprintf("azdo-cli-acc-%s", uuid.New().String()))
	t.Cleanup(func() {
		err := sharedProj.Cleanup()
		if err != nil {
			t.Logf("failed to delete project: %v", err)
		}
	})

	// Test Service Principal with Secret
	t.Run("ServicePrincipalWithSecret", func(t *testing.T) {
		t.Parallel()
		testAccCreateAzureRMServiceEndpoint(t, sharedProj, AuthSchemeServicePrincipal, CreationModeManual, func(opts *createOptions) {
			opts.servicePrincipalKey = "test-secret-123"
		})
	})

	// Test Service Principal with Certificate
	t.Run("ServicePrincipalWithCertificate", func(t *testing.T) {
		t.Parallel()
		testAccCreateAzureRMServiceEndpoint(t, sharedProj, AuthSchemeServicePrincipal, CreationModeManual, func(opts *createOptions) {
			opts.certificatePath = "test-cert.pem"
		})
	})

	// Test Managed Service Identity
	t.Run("ManagedServiceIdentity", func(t *testing.T) {
		t.Parallel()
		testAccCreateAzureRMServiceEndpoint(t, sharedProj, AuthSchemeManagedServiceIdentity, CreationModeManual, nil)
	})

	// Test Workload Identity Federation - Manual
	t.Run("WorkloadIdentityFederationManual", func(t *testing.T) {
		t.Parallel()
		testAccCreateAzureRMServiceEndpoint(t, sharedProj, AuthSchemeWorkloadIdentityFederation, CreationModeManual, nil)
	})

	// Test Workload Identity Federation - Automatic
	t.Run("WorkloadIdentityFederationAutomatic", func(t *testing.T) {
		t.Parallel()
		testAccCreateAzureRMServiceEndpoint(t, sharedProj, AuthSchemeWorkloadIdentityFederation, CreationModeAutomatic, nil)
	})
}

func testAccCreateAzureRMServiceEndpoint(t *testing.T, sharedProj *sharedProject, authScheme string, creationMode string, setupFunc func(*createOptions)) {
	// Generate unique names for each test run
	endpointName := fmt.Sprintf("azdo-cli-test-ep-%s-%s", authScheme, uuid.New().String())
	subscriptionID := uuid.New().String()
	subscriptionName := fmt.Sprintf("Test Subscription %s", authScheme)
	resourceGroup := fmt.Sprintf("test-rg-%s", uuid.New().String())

	inttest.Test(t, inttest.TestCase{
		Steps: []inttest.Step{
			{
				PreRun: func(ctx inttest.TestContext) error {
					return sharedProj.Ensure(ctx)
				},
				Run: func(ctx inttest.TestContext) error {
					projectName, err := getTestProjectName(ctx)
					if err != nil {
						return err
					}
					projectArg := fmt.Sprintf("%s/%s", ctx.Org(), projectName)

					opts := &createOptions{
						project:                       projectArg,
						name:                          endpointName,
						description:                   fmt.Sprintf("Test AzureRM endpoint with %s auth", authScheme),
						authenticationScheme:          authScheme,
						servicePrincipalID:            uuid.New().String(), // Random SPN ID
						servicePrincipalKey:           "",
						certificatePath:               "",
						tenantID:                      uuid.New().String(), // Random tenant ID
						subscriptionID:                subscriptionID,
						subscriptionName:              subscriptionName,
						resourceGroup:                 resourceGroup,
						environment:                   "AzureCloud",
						serviceEndpointCreationMode:   creationMode,
						grantPermissionToAllPipelines: true,
						yes:                           true,
					}

					if setupFunc != nil {
						setupFunc(opts)
					}

					if opts.certificatePath != "" {
						// create a temporary certificate file using the test helper
						certPath, err := inttest.WriteTestFileWithName(t, opts.certificatePath, strings.NewReader(testCertificatePEM))
						if err != nil {
							return fmt.Errorf("failed to write certificate file: %w", err)
						}
						// override the path in opts so the command uses the generated file
						opts.certificatePath = certPath
						ctx.SetValue(ctxKeyCertPath, certPath)
					}

					ctx.SetValue(ctxKeyCreateOpts, opts)

					// Execute the command
					return runCreate(ctx, opts)
				},
				Verify: func(ctx inttest.TestContext) error {
					storedOpts, ok := ctx.Value(ctxKeyCreateOpts)
					if !ok {
						return fmt.Errorf("test context missing create options")
					}
					opts := storedOpts.(*createOptions)

					client, err := ctx.ClientFactory().ServiceEndpoint(ctx.Context(), ctx.Org())
					if err != nil {
						return fmt.Errorf("failed to create service endpoint client: %w", err)
					}

					return inttest.Poll(func() error {
						projectName, err := getTestProjectName(ctx)
						if err != nil {
							return err
						}

						endpoints, err := client.GetServiceEndpoints(ctx.Context(), serviceendpoint.GetServiceEndpointsArgs{
							Project:        &projectName,
							Type:           types.ToPtr("azurerm"),
							IncludeDetails: types.ToPtr(true),
						})
						if err != nil {
							return fmt.Errorf("failed to list service endpoints: %w", err)
						}

						var foundEndpoint *serviceendpoint.ServiceEndpoint
						for _, ep := range *endpoints {
							if ep.Name != nil && *ep.Name == endpointName {
								foundEndpoint = &ep
								break
							}
						}

						if foundEndpoint == nil {
							return fmt.Errorf("service endpoint '%s' not found", endpointName)
						}

						if foundEndpoint.Id != nil {
							ctx.SetValue(ctxKeyEndpointID, foundEndpoint.Id.String())
						}
						if refs := foundEndpoint.ServiceEndpointProjectReferences; refs != nil {
							for _, ref := range *refs {
								if ref.ProjectReference != nil && ref.ProjectReference.Id != nil {
									ctx.SetValue(ctxKeyEndpointProjectID, ref.ProjectReference.Id.String())
									break
								}
							}
						}

						if foundEndpoint.Type == nil || *foundEndpoint.Type != "azurerm" {
							return fmt.Errorf("expected endpoint type 'azurerm', got '%s'", types.GetValue(foundEndpoint.Type, ""))
						}

						if foundEndpoint.Authorization == nil || foundEndpoint.Authorization.Scheme == nil {
							return fmt.Errorf("endpoint authorization scheme is nil")
						}

						if *foundEndpoint.Authorization.Scheme != authScheme {
							return fmt.Errorf("expected auth scheme '%s', got '%s'", authScheme, *foundEndpoint.Authorization.Scheme)
						}

						if foundEndpoint.Data == nil {
							return fmt.Errorf("endpoint data is nil")
						}

						data := *foundEndpoint.Data
						if _, ok := data["subscriptionId"]; !ok {
							return fmt.Errorf("subscriptionId not found in endpoint data")
						}
						if _, ok := data["subscriptionName"]; !ok {
							return fmt.Errorf("subscriptionName not found in endpoint data")
						}
						if _, ok := data["environment"]; !ok {
							return fmt.Errorf("environment not found in endpoint data")
						}

						if foundEndpoint.Authorization.Parameters == nil {
							return fmt.Errorf("endpoint authorization parameters is nil")
						}

						params := *foundEndpoint.Authorization.Parameters
						if _, ok := params["tenantid"]; !ok {
							return fmt.Errorf("tenantid not found in auth parameters")
						}

						switch authScheme {
						case AuthSchemeServicePrincipal:
							if _, ok := params["serviceprincipalid"]; !ok {
								return fmt.Errorf("serviceprincipalid not found in auth parameters")
							}
							if opts.servicePrincipalKey != "" {
								if _, ok := params["authenticationType"]; !ok || params["authenticationType"] != "spnKey" {
									return fmt.Errorf("expected authenticationType 'spnKey' for service principal with secret")
								}
							} else if opts.certificatePath != "" {
								if _, ok := params["authenticationType"]; !ok || params["authenticationType"] != "spnCertificate" {
									return fmt.Errorf("expected authenticationType 'spnCertificate' for service principal with certificate")
								}
							}
						case AuthSchemeWorkloadIdentityFederation:
							if creationMode == CreationModeManual {
								if _, ok := params["serviceprincipalid"]; !ok {
									return fmt.Errorf("serviceprincipalid not found in auth parameters for manual WIF")
								}
							}
						}

						return nil
					}, inttest.PollOptions{
						Tries:   10,
						Timeout: 30 * time.Second,
					})
				},
				PostRun: func(ctx inttest.TestContext) error {
					var errs []error

					if err := deleteCreatedEndpoint(ctx); err != nil {
						errs = append(errs, err)
					}

					if certVal, ok := ctx.Value(ctxKeyCertPath); ok {
						if certPath, _ := certVal.(string); strings.TrimSpace(certPath) != "" {
							if err := os.Remove(certPath); err != nil && !errors.Is(err, os.ErrNotExist) {
								errs = append(errs, fmt.Errorf("failed to remove certificate file: %w", err))
							}
						}
					}

					if len(errs) > 0 {
						return errors.Join(errs...)
					}
					return nil
				},
			},
		},
	})
}

func getTestProjectName(ctx inttest.TestContext) (string, error) {
	if val, ok := ctx.Value(ctxKeyProjectName); ok {
		if name, _ := val.(string); strings.TrimSpace(name) != "" {
			return name, nil
		}
	}
	if name := strings.TrimSpace(ctx.Project()); name != "" {
		return name, nil
	}
	return "", fmt.Errorf("test project name not available")
}

type sharedProject struct {
	name        string
	id          string
	initOnce    sync.Once
	cleanupOnce sync.Once
	ctx         inttest.TestContext
}

func newProject(name string) *sharedProject {
	return &sharedProject{name: name}
}

func (p *sharedProject) Ensure(ctx inttest.TestContext) error {
	var initErr error
	p.initOnce.Do(func() {
		projectID, err := provisionProject(ctx, p.name)
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
	ctx.SetValue(ctxKeyProjectName, p.name)
	ctx.SetValue(ctxKeyProjectID, p.id)
	return nil
}

func (p *sharedProject) Cleanup() error {
	var cleanupErr error
	p.cleanupOnce.Do(func() {
		cleanupErr = deleteProjectByID(p.ctx, p.id)
	})
	return cleanupErr
}

func provisionProject(ctx inttest.TestContext, projectName string) (string, error) {
	coreClient, err := ctx.ClientFactory().Core(ctx.Context(), ctx.Org())
	if err != nil {
		return "", fmt.Errorf("failed to create core client: %w", err)
	}

	teamProject := &core.TeamProject{
		Name: types.ToPtr(projectName),
	}
	vis := core.ProjectVisibilityValues.Private
	teamProject.Visibility = &vis

	processID, err := resolveProcessTemplate(ctx, coreClient, "Agile")
	if err != nil {
		return "", err
	}

	capabilities := map[string]map[string]string{
		"versioncontrol": {
			"sourceControlType": "Git",
		},
		"processTemplate": {
			"templateTypeId": processID,
		},
	}
	teamProject.Capabilities = &capabilities

	opRef, err := coreClient.QueueCreateProject(ctx.Context(), core.QueueCreateProjectArgs{
		ProjectToCreate: teamProject,
	})
	if err != nil {
		return "", fmt.Errorf("failed to queue project creation: %w", err)
	}

	if err := waitForOperation(ctx, opRef); err != nil {
		return "", fmt.Errorf("project creation failed: %w", err)
	}

	project, err := coreClient.GetProject(ctx.Context(), core.GetProjectArgs{
		ProjectId:           types.ToPtr(projectName),
		IncludeCapabilities: types.ToPtr(true),
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch created project %q: %w", projectName, err)
	}
	if project == nil || project.Id == nil {
		return "", fmt.Errorf("project %q returned empty id", projectName)
	}

	return project.Id.String(), nil
}

func deleteProjectByID(ctx inttest.TestContext, projectID string) error {
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
	return waitForOperation(ctx, op)
}

func deleteCreatedEndpoint(ctx inttest.TestContext) error {
	endpointVal, _ := ctx.Value(ctxKeyEndpointID)
	projectVal, _ := ctx.Value(ctxKeyEndpointProjectID)
	if projectVal == nil {
		projectVal, _ = ctx.Value(ctxKeyProjectID)
	}
	endpointID, _ := endpointVal.(string)
	projectID, _ := projectVal.(string)

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
	if err := client.DeleteServiceEndpoint(ctx.Context(), serviceendpoint.DeleteServiceEndpointArgs{
		EndpointId: &parsedEndpointID,
		ProjectIds: &projectIDs,
	}); err != nil {
		return fmt.Errorf("failed to delete service endpoint: %w", err)
	}
	return nil
}

func resolveProcessTemplate(ctx inttest.TestContext, client core.Client, preferred string) (string, error) {
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

func waitForOperation(ctx inttest.TestContext, opRef *operations.OperationReference) error {
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
