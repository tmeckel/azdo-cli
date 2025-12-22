package azurerm

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"

	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/test"
	inttest "github.com/tmeckel/azdo-cli/internal/test"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type contextKey string

const (
	ctxKeyCreateOpts contextKey = "azurerm/create-opts"
	ctxKeyCertPath   contextKey = "azurerm/cert-path"
)

const (
	testCertificatePEM = `-----BEGIN CERTIFICATE-----
MIIDXTCCAkWgAwIBAgIJAKoK/heBjcOuMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwHhcNMTIwOTEyMjE1MjAyWhcNMTUwOTEyMjE1MjAyWjBF
MQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50
ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBC
gKCAQEAwuTanj/Uo5Yhq7ckmL5jycB3Z/zPBuZjviQ4fAar/7xeOUe7/y2Kpls=
-----END CERTIFICATE-----`
)

func TestAccCreateAzureRMServiceEndpoint(t *testing.T) {
	t.Parallel()

	sharedProj := test.NewSharedProject(fmt.Sprintf("azdo-cli-acc-%s", uuid.New().String()))
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

func testAccCreateAzureRMServiceEndpoint(t *testing.T, sharedProj *test.SharedProject, authScheme string, creationMode string, setupFunc func(*createOptions)) {
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
					projectName, err := test.GetTestProjectName(ctx)
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
						projectName, err := test.GetTestProjectName(ctx)
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
							ctx.SetValue(test.CtxKeyEndpointID, foundEndpoint.Id.String())
						}
						if refs := foundEndpoint.ServiceEndpointProjectReferences; refs != nil {
							for _, ref := range *refs {
								if ref.ProjectReference != nil && ref.ProjectReference.Id != nil {
									ctx.SetValue(test.CtxKeyProjectID, ref.ProjectReference.Id.String())
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

					if err := test.CleanupEndpointFromContext(ctx, test.CtxKeyEndpointID, test.CtxKeyProjectID); err != nil {
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
