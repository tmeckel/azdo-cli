package update

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
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

func (d *deps) expectProjectScope(t *testing.T, org, project string) *mocks.MockGraphClient {
	coreClient := mocks.NewMockCoreClient(d.ctrl)
	graphClient := mocks.NewMockGraphClient(d.ctrl)
	d.fact.EXPECT().Core(gomock.Any(), org).Return(coreClient, nil).AnyTimes()
	d.fact.EXPECT().Graph(gomock.Any(), org).Return(graphClient, nil).AnyTimes()
	coreClient.EXPECT().GetProject(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetProjectArgs) (*core.TeamProject, error) {
			assert.Equal(t, project, *args.ProjectId)
			return &core.TeamProject{Id: types.ToPtr(uuid.New())}, nil
		}).AnyTimes()
	graphClient.EXPECT().GetDescriptor(gomock.Any(), gomock.Any()).
		Return(&graph.GraphDescriptorResult{Value: types.ToPtr("vssgp.project-scope")}, nil).AnyTimes()
	return graphClient
}

// expectGroup wires the Extensions client used by FindGroupByName. When
// graphClient is provided the factory's Graph expectation is skipped.
func (d *deps) expectGroup(org string, graphClient *mocks.MockGraphClient) *mocks.MockGraphClient {
	ext := mocks.NewMockAzDOExtension(d.ctrl)
	d.fact.EXPECT().Extensions(gomock.Any(), org).Return(ext, nil).AnyTimes()
	ext.EXPECT().FindGroupsByDisplayName(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*graph.GraphGroup{{
			Descriptor:  types.ToPtr("vssgp.Uy0xLTItMw=="),
			DisplayName: types.ToPtr("Developers"),
		}}, nil)

	if graphClient == nil {
		graphClient = mocks.NewMockGraphClient(d.ctrl)
		d.fact.EXPECT().Graph(gomock.Any(), org).Return(graphClient, nil).AnyTimes()
	}
	return graphClient
}

func execute(t *testing.T, d *deps, args ...string) error {
	t.Helper()
	cmd := NewCmd(d.cmd)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestRunUpdate_ExplicitOrgOrgLevelGroup(t *testing.T) {
	t.Parallel()

	d := setup(t)
	graphClient := d.expectGroup("MyOrg", nil)
	graphClient.EXPECT().UpdateGroup(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args graph.UpdateGroupArgs) (*graph.GraphGroup, error) {
			require.Equal(t, "vssgp.Uy0xLTItMw==", *args.GroupDescriptor)
			require.NotNil(t, args.PatchDocument)
			return &graph.GraphGroup{
				Descriptor:  types.ToPtr("vssgp.Uy0xLTItMw=="),
				DisplayName: types.ToPtr("New Name"),
			}, nil
		})

	err := execute(t, d, "MyOrg:/Developers", "--name", "New Name", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"name":"New Name"`)
}

func TestRunUpdate_ExplicitOrgProjectGroup(t *testing.T) {
	t.Parallel()

	d := setup(t)
	graphClient := d.expectProjectScope(t, "MyOrg", "MyProject")
	d.expectGroup("MyOrg", graphClient)
	graphClient.EXPECT().UpdateGroup(gomock.Any(), gomock.Any()).
		Return(&graph.GraphGroup{
			Descriptor:  types.ToPtr("vssgp.project-scope"),
			DisplayName: types.ToPtr("Developers"),
		}, nil)

	err := execute(t, d, "MyOrg:MyProject/Developers", "--description", "Updated description", "--json")
	require.NoError(t, err)
}

func TestRunUpdate_LegacySlashErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		arg     string
		wantErr string
	}{
		{
			name:    "legacy org slash group",
			arg:     "MyOrg/Developers",
			wantErr: "explicit organization is required, use ORG: syntax",
		},
		{
			name:    "legacy org slash project slash group",
			arg:     "MyOrg/MyProject/Developers",
			wantErr: "explicit organization is required, use ORG: syntax",
		},
		{
			name:    "bare group without ORG prefix",
			arg:     "Developers",
			wantErr: "explicit organization is required, use ORG: syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := setup(t)
			err := execute(t, d, tt.arg, "--name", "New Name", "--json")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNewCmd_UseDocumentsCanonicalSyntax(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	require.True(t, strings.HasPrefix(cmd.Use, "update ORG:[PROJECT/]GROUP"))
	assert.Contains(t, cmd.Example, "MyOrg:MyProject/Developers")
}
