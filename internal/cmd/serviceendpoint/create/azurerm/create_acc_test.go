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
	pollutil "github.com/tmeckel/azdo-cli/internal/util"
)

type contextKey string

const (
	ctxKeyCertPath contextKey = "azurerm/cert-path"
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

type createTestOptions struct {
	servicePrincipalKey string
	certificateFileName string
}

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
		testAccCreateAzureRMServiceEndpoint(t, sharedProj, AuthSchemeServicePrincipal, CreationModeManual, func(opts *createTestOptions) {
			opts.servicePrincipalKey = "test-secret-123"
		})
	})

	// Test Service Principal with Certificate
	t.Run("ServicePrincipalWithCertificate", func(t *testing.T) {
		t.Parallel()
		testAccCreateAzureRMServiceEndpoint(t, sharedProj, AuthSchemeServicePrincipal, CreationModeManual, func(opts *createTestOptions) {
			opts.certificateFileName = "test-cert.pem"
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

func testAccCreateAzureRMServiceEndpoint(t *testing.T, sharedProj *test.SharedProject, authScheme string, creationMode string, setupFunc func(*createTestOptions)) {
	// Generate unique names for each test run
	endpointName := fmt.Sprintf("azdo-cli-test-ep-%s-%s", authScheme, uuid.New().String())
	subscriptionID := uuid.New().String()
	subscriptionName := fmt.Sprintf("Test Subscription %s", authScheme)
	resourceGroup := fmt.Sprintf("test-rg-%s", uuid.New().String())

	testOpts := &createTestOptions{}
	if setupFunc != nil {
		setupFunc(testOpts)
	}

	inttest.Test(t, inttest.TestCase{
		AcceptanceTest: true,
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

					var certPath string
					if strings.TrimSpace(testOpts.certificateFileName) != "" {
						path, err := inttest.WriteTestFileWithName(t, testOpts.certificateFileName, strings.NewReader(testCertificatePEM))
						if err != nil {
							return fmt.Errorf("failed to write certificate file: %w", err)
						}
						certPath = path
						ctx.SetValue(ctxKeyCertPath, path)
					}

					var servicePrincipalID string
					switch authScheme {
					case AuthSchemeServicePrincipal:
						servicePrincipalID = uuid.New().String()
					case AuthSchemeWorkloadIdentityFederation:
						if creationMode == CreationModeManual {
							servicePrincipalID = uuid.New().String()
						}
					}

					args := []string{
						projectArg,
						"--name", endpointName,
						"--description", fmt.Sprintf("Test AzureRM endpoint with %s auth", authScheme),
						"--authentication-scheme", authScheme,
						"--tenant-id", uuid.New().String(),
						"--subscription-id", subscriptionID,
						"--subscription-name", subscriptionName,
						"--resource-group", resourceGroup,
						"--environment", "AzureCloud",
						"--grant-permission-to-all-pipelines",
					}

					if servicePrincipalID != "" {
						args = append(args, "--service-principal-id", servicePrincipalID)
					}
					if authScheme == AuthSchemeServicePrincipal {
						if strings.TrimSpace(testOpts.servicePrincipalKey) != "" {
							args = append(args, "--service-principal-key", testOpts.servicePrincipalKey)
						} else if certPath != "" {
							args = append(args, "--certificate-path", certPath)
						}
					}

					cmd := NewCmd(ctx)
					cmd.SetArgs(args)
					return cmd.Execute()
				},
				Verify: func(ctx inttest.TestContext) error {
					client, err := ctx.ClientFactory().ServiceEndpoint(ctx.Context(), ctx.Org())
					if err != nil {
						return fmt.Errorf("failed to create service endpoint client: %w", err)
					}

					return pollutil.Poll(ctx.Context(), func() error {
						projectName, err := test.GetTestProjectName(ctx)
						if err != nil {
							return err
						}

						endpoints, err := client.GetServiceEndpoints(ctx.Context(), serviceendpoint.GetServiceEndpointsArgs{
							Project:        &projectName,
							Type:           types.ToPtr("azurerm"),
							IncludeDetails: types.ToPtr(true),
							IncludeFailed:  types.ToPtr(true),
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
							if strings.TrimSpace(testOpts.servicePrincipalKey) != "" {
								if _, ok := params["authenticationType"]; !ok || params["authenticationType"] != "spnKey" {
									return fmt.Errorf("expected authenticationType 'spnKey' for service principal with secret")
								}
							} else if strings.TrimSpace(testOpts.certificateFileName) != "" {
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
					}, pollutil.PollOptions{
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
