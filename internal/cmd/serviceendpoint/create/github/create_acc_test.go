package github

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"

	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/test"
	inttest "github.com/tmeckel/azdo-cli/internal/test"
	"github.com/tmeckel/azdo-cli/internal/types"
	pollutil "github.com/tmeckel/azdo-cli/internal/util"
)

func TestAccCreateGitHubServiceEndpoint(t *testing.T) {
	t.Parallel()

	sharedProj := test.NewSharedProject(fmt.Sprintf("azdo-cli-acc-%s", uuid.New().String()))

	t.Cleanup(func() {
		_ = sharedProj.Cleanup()
	})

	t.Run("PersonalAccessToken", func(t *testing.T) {
		t.Parallel()

		endpointName := fmt.Sprintf("azdo-cli-acc-gh-%s", uuid.New().String())

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

						cmd := NewCmd(ctx)
						cmd.SetArgs([]string{
							projectArg,
							"--name", endpointName,
							"--url", "https://github.com",
							"--token", uuid.New().String(),
						})
						return cmd.Execute()
					},
					Verify: func(ctx inttest.TestContext) error {
						client, err := ctx.ClientFactory().ServiceEndpoint(ctx.Context(), ctx.Org())
						if err != nil {
							return err
						}

						return pollutil.Poll(ctx.Context(), func() error {
							projectName, err := test.GetTestProjectName(ctx)
							if err != nil {
								return err
							}

							endpoints, err := client.GetServiceEndpoints(ctx.Context(), serviceendpoint.GetServiceEndpointsArgs{
								Project:        &projectName,
								Type:           types.ToPtr("github"),
								IncludeDetails: types.ToPtr(true),
							})
							if err != nil {
								return err
							}

							var found *serviceendpoint.ServiceEndpoint
							for _, ep := range *endpoints {
								if ep.Name != nil && *ep.Name == endpointName {
									found = &ep
									break
								}
							}
							if found == nil {
								return fmt.Errorf("service endpoint '%s' not found", endpointName)
							}

							if found.Id != nil {
								ctx.SetValue(test.CtxKeyEndpointID, found.Id.String())
							}
							if refs := found.ServiceEndpointProjectReferences; refs != nil {
								for _, r := range *refs {
									if r.ProjectReference != nil && r.ProjectReference.Id != nil {
										ctx.SetValue(test.CtxKeyProjectID, r.ProjectReference.Id.String())
										break
									}
								}
							}

							if found.Type == nil || *found.Type != "github" {
								return fmt.Errorf("expected endpoint type 'github', got '%s'", types.GetValue(found.Type, ""))
							}

							// Authorization may be present; ensure scheme is set
							if found.Authorization == nil || found.Authorization.Scheme == nil {
								return fmt.Errorf("endpoint authorization scheme is nil")
							}

							return nil
						}, pollutil.PollOptions{
							Tries:   10,
							Timeout: 30 * time.Second,
						})
					},
					PostRun: func(ctx inttest.TestContext) error {
						return test.CleanupEndpointFromContext(ctx, test.CtxKeyEndpointID, test.CtxKeyProjectID)
					},
				},
			},
		})
	})

	t.Run("ConfigurationID", func(t *testing.T) {
		t.Parallel()

		endpointName := fmt.Sprintf("azdo-cli-acc-gh-cfg-%s", uuid.New().String())

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

						cmd := NewCmd(ctx)
						cmd.SetArgs([]string{
							projectArg,
							"--name", endpointName,
							"--url", "https://github.com",
							"--configuration-id", uuid.New().String(),
						})
						return cmd.Execute()
					},
					Verify: func(ctx inttest.TestContext) error {
						client, err := ctx.ClientFactory().ServiceEndpoint(ctx.Context(), ctx.Org())
						if err != nil {
							return err
						}

						return pollutil.Poll(ctx.Context(), func() error {
							projectName, err := test.GetTestProjectName(ctx)
							if err != nil {
								return err
							}

							endpoints, err := client.GetServiceEndpoints(ctx.Context(), serviceendpoint.GetServiceEndpointsArgs{
								Project:        &projectName,
								Type:           types.ToPtr("github"),
								IncludeDetails: types.ToPtr(true),
							})
							if err != nil {
								return err
							}

							var found *serviceendpoint.ServiceEndpoint
							for _, ep := range *endpoints {
								if ep.Name != nil && *ep.Name == endpointName {
									found = &ep
									break
								}
							}
							if found == nil {
								return fmt.Errorf("service endpoint '%s' not found", endpointName)
							}

							if found.Id != nil {
								ctx.SetValue(test.CtxKeyEndpointID, found.Id.String())
							}
							if refs := found.ServiceEndpointProjectReferences; refs != nil {
								for _, r := range *refs {
									if r.ProjectReference != nil && r.ProjectReference.Id != nil {
										ctx.SetValue(test.CtxKeyProjectID, r.ProjectReference.Id.String())
										break
									}
								}
							}

							if found.Type == nil || *found.Type != "github" {
								return fmt.Errorf("expected endpoint type 'github', got '%s'", types.GetValue(found.Type, ""))
							}

							// Authorization may be present; ensure scheme is set
							if found.Authorization == nil || found.Authorization.Scheme == nil {
								return fmt.Errorf("endpoint authorization scheme is nil")
							}

							return nil
						}, pollutil.PollOptions{
							Tries:   10,
							Timeout: 30 * time.Second,
						})
					},
					PostRun: func(ctx inttest.TestContext) error {
						return test.CleanupEndpointFromContext(ctx, test.CtxKeyEndpointID, test.CtxKeyProjectID)
					},
				},
			},
		})
	})
}
