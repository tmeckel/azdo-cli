package create

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type deps struct {
	ctrl *gomock.Controller
	cmd  *mocks.MockCmdContext
	fact *mocks.MockClientFactory
	ios  *iostreams.IOStreams
	in   *bytes.Buffer
	out  *bytes.Buffer
}

const endpointPayload = `{"name":"MyConnection","type":"generic","url":"https://example.com"}`

func setup(t *testing.T) *deps {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, in, out, _ := iostreams.Test()

	d := &deps{
		ctrl: ctrl,
		cmd:  mocks.NewMockCmdContext(ctrl),
		fact: mocks.NewMockClientFactory(ctrl),
		ios:  io,
		in:   in,
		out:  out,
	}
	d.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	d.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	d.cmd.EXPECT().ClientFactory().Return(d.fact).AnyTimes()
	return d
}

func (d *deps) setupDefaultOrg(org string) {
	cfg := mocks.NewMockConfig(d.ctrl)
	auth := mocks.NewMockAuthConfig(d.ctrl)
	d.cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(auth).AnyTimes()
	auth.EXPECT().GetDefaultOrganization().Return(org, nil).AnyTimes()
}

func (d *deps) expectCreate(t *testing.T, org, project string) {
	coreClient := mocks.NewMockCoreClient(d.ctrl)
	d.fact.EXPECT().Core(gomock.Any(), org).Return(coreClient, nil).AnyTimes()
	coreClient.EXPECT().GetProject(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetProjectArgs) (*core.TeamProject, error) {
			assert.Equal(t, project, *args.ProjectId)
			return &core.TeamProject{
				Id:   types.ToPtr(uuid.MustParse("00000000-0000-0000-0000-000000000001")),
				Name: types.ToPtr(project),
			}, nil
		})

	client := mocks.NewMockServiceEndpointClient(d.ctrl)
	d.fact.EXPECT().ServiceEndpoint(gomock.Any(), org).Return(client, nil).AnyTimes()
	client.EXPECT().CreateServiceEndpoint(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args serviceendpoint.CreateServiceEndpointArgs) (*serviceendpoint.ServiceEndpoint, error) {
			assert.Equal(t, "MyConnection", *args.Endpoint.Name)
			return &serviceendpoint.ServiceEndpoint{
				Id:   types.ToPtr(uuid.MustParse("00000000-0000-0000-0000-000000000001")),
				Name: types.ToPtr("MyConnection"),
			}, nil
		})
}

func execute(t *testing.T, d *deps, args ...string) error {
	t.Helper()
	cmd := NewCmd(d.cmd)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestRunCreateFromFile_DefaultOrgProjectScope(t *testing.T) {
	t.Parallel()

	d := setup(t)
	d.setupDefaultOrg("myorg")
	fmt.Fprintln(d.in, endpointPayload)
	d.expectCreate(t, "myorg", "MyProject")

	err := execute(t, d, "MyProject", "--from-file", "-", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"name":"MyConnection"`)
}

func TestRunCreateFromFile_ExplicitOrgProjectScope(t *testing.T) {
	t.Parallel()

	d := setup(t)
	fmt.Fprintln(d.in, endpointPayload)
	d.expectCreate(t, "otherorg", "MyProject")

	err := execute(t, d, "otherorg:MyProject", "--from-file", "-", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"name":"MyConnection"`)
}

func TestRunCreateFromFile_LegacySlashErrorAndReinterpretation(t *testing.T) {
	t.Parallel()

	d := setup(t)
	err := execute(t, d, "otherorg/MyProject", "--from-file", "-")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "legacy ORGANIZATION/... form is not supported, use ORG: syntax")

	d2 := setup(t)
	d2.setupDefaultOrg("myorg")
	fmt.Fprintln(d2.in, endpointPayload)
	d2.expectCreate(t, "myorg", "otherorg")

	err = execute(t, d2, "otherorg", "--from-file", "-")
	require.NoError(t, err)
}

func TestNewCmd_UseDocumentsCanonicalSyntax(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	require.True(t, strings.HasPrefix(cmd.Use, "create [ORG:]PROJECT --from-file"))
}
