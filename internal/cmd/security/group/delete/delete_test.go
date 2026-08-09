package delete

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

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
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

// expectGroup wires the Extensions client used by FindGroupByName. When
// graphClient is nil a fresh Graph client mock is created and returned for
// the delete call; otherwise the provided client is reused.
func (d *deps) expectGroup(org string, graphClient *mocks.MockGraphClient) *mocks.MockGraphClient {
	ext := mocks.NewMockAzDOExtension(d.ctrl)
	d.fact.EXPECT().Extensions(gomock.Any(), org).Return(ext, nil).AnyTimes()
	ext.EXPECT().FindGroupsByDisplayName(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*graph.GraphGroup{{
			Descriptor:  types.ToPtr("vssgp.Uy0xLTItMw=="),
			DisplayName: types.ToPtr("GroupName"),
		}}, nil)

	if graphClient == nil {
		graphClient = mocks.NewMockGraphClient(d.ctrl)
		d.fact.EXPECT().Graph(gomock.Any(), org).Return(graphClient, nil).AnyTimes()
	}
	return graphClient
}

// expectProjectScope wires the Core/Graph clients used by ResolveScopeDescriptor
// and returns the Graph client so callers can attach further expectations.
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

func execute(t *testing.T, d *deps, args ...string) error {
	t.Helper()
	cmd := NewCmd(d.cmd)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestRunDelete_ExplicitOrgOrgLevelGroup(t *testing.T) {
	t.Parallel()

	d := setup(t)
	graphClient := d.expectGroup("MyOrg", nil)
	graphClient.EXPECT().DeleteGroup(gomock.Any(), graph.DeleteGroupArgs{
		GroupDescriptor: types.ToPtr("vssgp.Uy0xLTItMw=="),
	}).Return(nil)

	err := execute(t, d, "MyOrg:/GroupName", "--yes")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `Deleted security group "GroupName"`)
}

func TestRunDelete_ExplicitOrgProjectGroup(t *testing.T) {
	t.Parallel()

	d := setup(t)
	graphClient := d.expectProjectScope(t, "MyOrg", "MyProject")
	d.expectGroup("MyOrg", graphClient)
	graphClient.EXPECT().DeleteGroup(gomock.Any(), gomock.Any()).Return(nil)

	err := execute(t, d, "MyOrg:MyProject/GroupName", "--yes")
	require.NoError(t, err)
}

func TestRunDelete_PromptConfirmsWithoutYesFlag(t *testing.T) {
	t.Parallel()

	d := setup(t)
	graphClient := d.expectGroup("MyOrg", nil)
	graphClient.EXPECT().DeleteGroup(gomock.Any(), gomock.Any()).Return(nil)

	prompter := mocks.NewMockPrompter(d.ctrl)
	d.cmd.EXPECT().Prompter().Return(prompter, nil).AnyTimes()
	prompter.EXPECT().Confirm(gomock.Any(), false).Return(true, nil)

	err := execute(t, d, "MyOrg:/GroupName")
	require.NoError(t, err)
}

func TestRunDelete_PromptDeclinedReturnsCancel(t *testing.T) {
	t.Parallel()

	d := setup(t)
	d.expectGroup("MyOrg", nil)

	prompter := mocks.NewMockPrompter(d.ctrl)
	d.cmd.EXPECT().Prompter().Return(prompter, nil).AnyTimes()
	prompter.EXPECT().Confirm(gomock.Any(), false).Return(false, nil)

	err := execute(t, d, "MyOrg:/GroupName")
	require.ErrorIs(t, err, util.ErrCancel)
}

func TestRunDelete_LegacySlashErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		arg     string
		wantErr string
	}{
		{
			name:    "legacy org slash group",
			arg:     "MyOrg/GroupName",
			wantErr: "explicit organization is required, use ORG: syntax",
		},
		{
			name:    "legacy org slash project slash group",
			arg:     "MyOrg/MyProject/GroupName",
			wantErr: "explicit organization is required, use ORG: syntax",
		},
		{
			name:    "leading slash without ORG prefix",
			arg:     "/GroupName",
			wantErr: "explicit organization is required, use ORG: syntax",
		},
		{
			name:    "org marker without target",
			arg:     "MyOrg:",
			wantErr: "use ORG:/ to specify an organization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := setup(t)
			err := execute(t, d, tt.arg, "--yes")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNewCmd_UseDocumentsCanonicalSyntax(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	require.True(t, strings.HasPrefix(cmd.Use, "delete ORG:[PROJECT/]GROUP"))
}
