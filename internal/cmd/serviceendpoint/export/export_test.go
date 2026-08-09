package export

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
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
	out  *bytes.Buffer
}

func setup(t *testing.T) *deps {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()

	d := &deps{
		ctrl: ctrl,
		cmd:  mocks.NewMockCmdContext(ctrl),
		fact: mocks.NewMockClientFactory(ctrl),
		ios:  io,
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

func (d *deps) expectEndpoint(t *testing.T, org, project, name string) {
	client := mocks.NewMockServiceEndpointClient(d.ctrl)
	d.fact.EXPECT().ServiceEndpoint(gomock.Any(), org).Return(client, nil).AnyTimes()
	client.EXPECT().GetServiceEndpointsByNames(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args serviceendpoint.GetServiceEndpointsByNamesArgs) (*[]serviceendpoint.ServiceEndpoint, error) {
			assert.Equal(t, project, *args.Project)
			require.Len(t, *args.EndpointNames, 1)
			assert.Equal(t, name, (*args.EndpointNames)[0])
			return &[]serviceendpoint.ServiceEndpoint{{
				Id:       types.ToPtr(uuid.MustParse("00000000-0000-0000-0000-000000000001")),
				Name:     types.ToPtr("MyConnection"),
				Type:     types.ToPtr("generic"),
				Url:      types.ToPtr("https://example.com"),
				IsShared: types.ToPtr(false),
			}}, nil
		})
}

func execute(t *testing.T, d *deps, args ...string) error {
	t.Helper()
	cmd := NewCmd(d.cmd)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestRunExport_DefaultOrgProjectFirstTarget(t *testing.T) {
	t.Parallel()

	d := setup(t)
	d.setupDefaultOrg("myorg")
	d.expectEndpoint(t, "myorg", "MyProject", "MyConnection")

	err := execute(t, d, "MyProject/MyConnection")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"name": "MyConnection"`)
}

func TestRunExport_ExplicitOrgTarget(t *testing.T) {
	t.Parallel()

	d := setup(t)
	d.expectEndpoint(t, "otherorg", "MyProject", "MyConnection")

	err := execute(t, d, "otherorg:MyProject/MyConnection")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"name": "MyConnection"`)
}

func TestRunExport_LegacySlashErrorAndReinterpretation(t *testing.T) {
	t.Parallel()

	d := setup(t)
	err := execute(t, d, "otherorg/MyProject/MyConnection")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "legacy ORGANIZATION/... form is not supported, use ORG: syntax")

	d2 := setup(t)
	d2.setupDefaultOrg("myorg")
	d2.expectEndpoint(t, "myorg", "otherorg", "MyConnection")

	err = execute(t, d2, "otherorg/MyConnection")
	require.NoError(t, err)
	assert.Contains(t, d2.out.String(), `"name": "MyConnection"`)
}

func TestNewCmd_UseDocumentsCanonicalSyntax(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	require.True(t, strings.HasPrefix(cmd.Use, "export [ORG:]PROJECT/ID_OR_NAME"))
}
