package list

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type dependencies struct {
	ctrl       *gomock.Controller
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	graphCli   *mocks.MockGraphClient
	coreCli    *mocks.MockCoreClient
	config     *mocks.MockConfig
	auth       *mocks.MockAuthConfig
	stdout     *bytes.Buffer
}

func newDependencies(t *testing.T) *dependencies {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &dependencies{
		ctrl:       ctrl,
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		graphCli:   mocks.NewMockGraphClient(ctrl),
		coreCli:    mocks.NewMockCoreClient(ctrl),
		stdout:     out,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()

	return deps
}

func (d *dependencies) setupDefaultOrg(org string) {
	d.config = mocks.NewMockConfig(d.ctrl)
	d.auth = mocks.NewMockAuthConfig(d.ctrl)
	d.cmd.EXPECT().Config().Return(d.config, nil).AnyTimes()
	d.config.EXPECT().Authentication().Return(d.auth).AnyTimes()
	d.auth.EXPECT().GetDefaultOrganization().Return(org, nil).AnyTimes()
}

func (d *dependencies) setupNoDefaultOrg() {
	d.config = mocks.NewMockConfig(d.ctrl)
	d.auth = mocks.NewMockAuthConfig(d.ctrl)
	d.cmd.EXPECT().Config().Return(d.config, nil).AnyTimes()
	d.config.EXPECT().Authentication().Return(d.auth).AnyTimes()
	d.auth.EXPECT().GetDefaultOrganization().Return("", nil).AnyTimes()
}

func sampleUsers() *graph.PagedGraphUsers {
	return &graph.PagedGraphUsers{
		GraphUsers: &[]graph.GraphUser{
			{
				Descriptor:    types.ToPtr("aad.MzE5OQ"),
				DisplayName:   types.ToPtr("John Doe"),
				PrincipalName: types.ToPtr("john.doe@example.com"),
				MailAddress:   types.ToPtr("john.doe@example.com"),
			},
		},
	}
}

// expectGraphClient wires the Graph client factory and the ListUsers response,
// asserting the organization routing and the requested scope descriptor.
func (d *dependencies) expectGraphClient(t *testing.T, wantOrg, wantScopeDescriptor string) {
	t.Helper()
	d.clientFact.EXPECT().Graph(gomock.Any(), wantOrg).Return(d.graphCli, nil).AnyTimes()
	d.graphCli.EXPECT().ListUsers(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args graph.ListUsersArgs) (*graph.PagedGraphUsers, error) {
			require.NotNil(t, args.SubjectTypes)
			assert.Equal(t, []string{"aad"}, *args.SubjectTypes)
			require.NotNil(t, args.ScopeDescriptor)
			assert.Equal(t, wantScopeDescriptor, *args.ScopeDescriptor)
			return sampleUsers(), nil
		})
}

// expectProjectScope wires the Core/Graph calls used by ResolveScopeDescriptor
// to resolve the project scope descriptor.
func (d *dependencies) expectProjectScope(t *testing.T, wantOrg, projectName, descriptor string) {
	t.Helper()
	projectID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	d.clientFact.EXPECT().Core(gomock.Any(), wantOrg).Return(d.coreCli, nil).AnyTimes()
	d.coreCli.EXPECT().GetProject(gomock.Any(), gomock.AssignableToTypeOf(core.GetProjectArgs{})).
		DoAndReturn(func(_ context.Context, args core.GetProjectArgs) (*core.TeamProject, error) {
			require.NotNil(t, args.ProjectId)
			assert.Equal(t, projectName, *args.ProjectId)
			return &core.TeamProject{Id: types.ToPtr(projectID)}, nil
		})
	d.graphCli.EXPECT().GetDescriptor(gomock.Any(), gomock.AssignableToTypeOf(graph.GetDescriptorArgs{})).
		Return(&graph.GraphDescriptorResult{Value: types.ToPtr(descriptor)}, nil)
}

func TestRunCmd_ScopeRouting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		orgFlag     string
		projectArg  string
		defaultOrg  string
		wantOrg     string
		wantProject string
		wantErr     string
	}{
		{
			name:       "default org routing",
			defaultOrg: "default-org",
			wantOrg:    "default-org",
		},
		{
			name:    "explicit org via flag",
			orgFlag: "myorg",
			wantOrg: "myorg",
		},
		{
			name:        "project scope with default org",
			projectArg:  "MyProject",
			defaultOrg:  "default-org",
			wantOrg:     "default-org",
			wantProject: "MyProject",
		},
		{
			name:        "composed org and project",
			orgFlag:     "myorg",
			projectArg:  "MyProject",
			wantOrg:     "myorg",
			wantProject: "MyProject",
		},
		{
			name:       "legacy slash project rejected",
			projectArg: "myorg/MyProject",
			defaultOrg: "default-org",
			wantErr:    "legacy ORGANIZATION/... form is not supported, use ORG: syntax",
		},
		{
			name:    "no organization configured",
			wantErr: "no organization specified and no default organization configured",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t)
			switch {
			case tt.wantErr == "no organization specified and no default organization configured":
				deps.setupNoDefaultOrg()
			case tt.defaultOrg != "":
				deps.setupDefaultOrg(tt.defaultOrg)
			}
			if tt.wantErr == "" {
				scopeDescriptor := ""
				if tt.wantProject != "" {
					scopeDescriptor = "vssgp.MyProject"
					deps.expectProjectScope(t, tt.wantOrg, tt.wantProject, scopeDescriptor)
				}
				deps.expectGraphClient(t, tt.wantOrg, scopeDescriptor)
				tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
				require.NoError(t, err)
				deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()
			}

			err := runCmd(deps.cmd, &usersListOptions{
				organizationName: tt.orgFlag,
				projectName:      tt.projectArg,
				top:              20,
			})
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, deps.stdout.String(), "John Doe")
		})
	}
}

func TestNewCmd_ProjectArgWiring(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.setupDefaultOrg("default-org")
	deps.expectProjectScope(t, "default-org", "MyProject", "vssgp.MyProject")
	deps.expectGraphClient(t, "default-org", "vssgp.MyProject")
	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"MyProject"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, deps.stdout.String(), "John Doe")
}

func TestNewCmd_LegacySlashArgError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.setupDefaultOrg("default-org")

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myorg/MyProject"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "legacy ORGANIZATION/... form is not supported, use ORG: syntax")
}
