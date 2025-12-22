package delete

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"

	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/test"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	inttest "github.com/tmeckel/azdo-cli/internal/test"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type ctxKey string

const (
	ctxKeyEndpointName ctxKey = "serviceendpoint/delete/endpoint-name"
)

func TestAccDeleteServiceEndpoint(t *testing.T) {
	t.Parallel()

	sharedProj := test.NewSharedProject(fmt.Sprintf("azdo-cli-acc-del-%s", uuid.New().String()))
	t.Cleanup(func() {
		if err := sharedProj.Cleanup(); err != nil {
			t.Logf("failed to cleanup project: %v", err)
		}
	})

	inttest.Test(t, inttest.TestCase{
		Steps: []inttest.Step{
			{
				PreRun: func(ctx inttest.TestContext) error {
					if err := sharedProj.Ensure(ctx); err != nil {
						return err
					}
					projectName, err := test.GetTestProjectName(ctx)
					if err != nil {
						return err
					}
					projectIDVal, _ := ctx.Value(test.CtxKeyProjectID)
					projectID, _ := projectIDVal.(string)
					endpointID, endpointName, err := test.CreateTestServiceEndpoint(ctx, projectName, projectID)
					if err != nil {
						return err
					}
					ctx.SetValue(test.CtxKeyEndpointID, endpointID)
					ctx.SetValue(ctxKeyEndpointName, endpointName)
					return nil
				},
				Run: func(ctx inttest.TestContext) error {
					projectName, err := test.GetTestProjectName(ctx)
					if err != nil {
						return err
					}
					endpointNameAny, _ := ctx.Value(ctxKeyEndpointName)
					endpointName, _ := endpointNameAny.(string)
					if projectName == "" || endpointName == "" {
						return fmt.Errorf("missing project or endpoint name in context")
					}
					target := fmt.Sprintf("%s/%s/%s", ctx.Org(), projectName, endpointName)
					opts := &deleteOptions{
						targetArg: target,
						yes:       true,
					}
					return runDelete(ctx, opts)
				},
				Verify: func(ctx inttest.TestContext) error {
					projectName, err := test.GetTestProjectName(ctx)
					if err != nil {
						return err
					}
					endpointIDAny, _ := ctx.Value(test.CtxKeyEndpointID)
					endpointIDStr, _ := endpointIDAny.(string)
					if projectName == "" || endpointIDStr == "" {
						return fmt.Errorf("missing verification context values")
					}
					client, err := ctx.ClientFactory().ServiceEndpoint(ctx.Context(), ctx.Org())
					if err != nil {
						return err
					}
					endpointID, err := uuid.Parse(endpointIDStr)
					if err != nil {
						return fmt.Errorf("invalid endpoint id: %w", err)
					}
					return inttest.Poll(func() error {
						sp, err := client.GetServiceEndpointDetails(ctx.Context(), serviceendpoint.GetServiceEndpointDetailsArgs{
							Project:    types.ToPtr(projectName),
							EndpointId: &endpointID,
						})
						if err == nil {
							if sp != nil && sp.Id != nil { // GetServiceEndpointDetails returns an empty result instead of nil or an HTTP 404
								return fmt.Errorf("service endpoint still exists")
							}
						} else if util.IsNotFoundError(err) {
							return nil
						}
						return err
					}, inttest.PollOptions{
						Tries:   10,
						Timeout: 240 * time.Second,
					})
				},
			},
		},
	})
}
