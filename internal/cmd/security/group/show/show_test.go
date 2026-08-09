package show

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

// expectProjectScope wires the Core/Graph clients used by ResolveScopeDescriptor.
func (d *deps) expectProjectScope(t *testing.T, org, project string) {
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
}

func execute(t *testing.T, d *deps, args ...string) error {
	t.Helper()
	cmd := NewCmd(d.cmd)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestRunShow_ExplicitOrgOrgLevelGroup(t *testing.T) {
	t.Parallel()

	d := setup(t)
	ext := mocks.NewMockAzDOExtension(d.ctrl)
	d.fact.EXPECT().Extensions(gomock.Any(), "MyOrg").Return(ext, nil).AnyTimes()
	ext.EXPECT().FindGroupsByDisplayName(gomock.Any(), "Contributors", nil).
		Return([]*graph.GraphGroup{{
			Descriptor:  types.ToPtr("vssgp.Uy0xLTItMw=="),
			DisplayName: types.ToPtr("Contributors"),
		}}, nil)

	err := execute(t, d, "MyOrg:/Contributors", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"descriptor":"vssgp.Uy0xLTItMw=="`)
	assert.Contains(t, d.out.String(), `"name":"Contributors"`)
}

func TestRunShow_ExplicitOrgProjectGroup(t *testing.T) {
	t.Parallel()

	d := setup(t)
	d.expectProjectScope(t, "MyOrg", "MyProject")
	ext := mocks.NewMockAzDOExtension(d.ctrl)
	d.fact.EXPECT().Extensions(gomock.Any(), "MyOrg").Return(ext, nil).AnyTimes()
	ext.EXPECT().FindGroupsByDisplayName(gomock.Any(), "Contributors", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, scopeDescriptor *string) ([]*graph.GraphGroup, error) {
			require.NotNil(t, scopeDescriptor)
			assert.Equal(t, "vssgp.project-scope", *scopeDescriptor)
			return []*graph.GraphGroup{{
				Descriptor:  types.ToPtr("vssgp.project-scope"),
				DisplayName: types.ToPtr("Contributors"),
			}}, nil
		})

	err := execute(t, d, "MyOrg:MyProject/Contributors", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"name":"Contributors"`)
}

func TestRunShow_LegacySlashErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		arg     string
		wantErr string
	}{
		{
			name:    "legacy org slash group",
			arg:     "MyOrg/Contributors",
			wantErr: "explicit organization is required, use ORG: syntax",
		},
		{
			name:    "legacy org slash project slash group",
			arg:     "MyOrg/MyProject/Contributors",
			wantErr: "explicit organization is required, use ORG: syntax",
		},
		{
			name:    "bare group without ORG prefix",
			arg:     "Contributors",
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
			err := execute(t, d, tt.arg, "--json")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestRunShow_TableOutputWithoutJSON(t *testing.T) {
	t.Parallel()

	d := setup(t)
	ext := mocks.NewMockAzDOExtension(d.ctrl)
	d.fact.EXPECT().Extensions(gomock.Any(), "MyOrg").Return(ext, nil).AnyTimes()
	ext.EXPECT().FindGroupsByDisplayName(gomock.Any(), "Contributors", nil).
		Return([]*graph.GraphGroup{{
			Descriptor:  types.ToPtr("vssgp.Uy0xLTItMw=="),
			DisplayName: types.ToPtr("Contributors"),
		}}, nil)

	printer := mocks.NewMockPrinter(d.ctrl)
	d.cmd.EXPECT().Printer("list").Return(printer, nil).AnyTimes()
	printer.EXPECT().AddColumns(gomock.Any()).AnyTimes()
	printer.EXPECT().AddField(gomock.Any()).AnyTimes()
	printer.EXPECT().EndRow().AnyTimes()
	printer.EXPECT().Render().Return(nil).AnyTimes()

	err := execute(t, d, "MyOrg:/Contributors")
	require.NoError(t, err)
}

func TestRunShow_OutputContainsCanonicalHelpSyntax(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	require.True(t, strings.HasPrefix(cmd.Use, "show ORG:[PROJECT/]GROUP"))
	assert.Contains(t, cmd.Example, "MyOrg:/Project Collection Administrators")
}
