package list

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type dependencies struct {
	ctrl        *gomock.Controller
	cmd         *mocks.MockCmdContext
	clientFact  *mocks.MockClientFactory
	connFactory *mocks.MockConnectionFactory
	gitClient   *mocks.MockAzDOGitClient
	config      *mocks.MockConfig
	auth        *mocks.MockAuthConfig
	stdout      *bytes.Buffer
}

func newDependencies(t *testing.T) *dependencies {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &dependencies{
		ctrl:        ctrl,
		cmd:         mocks.NewMockCmdContext(ctrl),
		clientFact:  mocks.NewMockClientFactory(ctrl),
		connFactory: mocks.NewMockConnectionFactory(ctrl),
		gitClient:   mocks.NewMockAzDOGitClient(ctrl),
		stdout:      out,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.cmd.EXPECT().ConnectionFactory().Return(deps.connFactory).AnyTimes()

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

func sampleRepos() *[]git.GitRepository {
	repoID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	return &[]git.GitRepository{
		{
			Id:            &repoID,
			Name:          types.ToPtr("demo-repo"),
			SshUrl:        types.ToPtr("git@ssh.dev.azure.com:v3/myorg/Fabrikam/demo-repo"),
			WebUrl:        types.ToPtr("https://dev.azure.com/myorg/Fabrikam/_git/demo-repo"),
			DefaultBranch: types.ToPtr("refs/heads/main"),
		},
	}
}

func TestNewCmd_ScopeHelp(t *testing.T) {
	t.Parallel()

	cmd := NewCmdRepoList(nil)
	assert.Equal(t, "list [ORG:]PROJECT", cmd.Use)
	assert.ElementsMatch(t, []string{"ls", "l"}, cmd.Aliases)
	assert.NotNil(t, cmd.RunE)
}

func TestNewCmd_MissingProjectArg(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.setupDefaultOrg("myorg")

	cmd := NewCmdRepoList(deps.cmd)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot list: project name required")
}

func TestRunList_ScopeRouting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		defaultOrg string
		wantOrg    string
		wantErr    string
	}{
		{
			name:       "default org routing",
			args:       []string{"myproject"},
			defaultOrg: "default-org",
			wantOrg:    "default-org",
		},
		{
			name:    "explicit org routing",
			args:    []string{"myorg:myproject"},
			wantOrg: "myorg",
		},
		{
			name:    "legacy org slash rejected",
			args:    []string{"myorg/myproject"},
			wantErr: "legacy ORGANIZATION/... form is not supported, use ORG: syntax",
		},
		{
			name:    "no default organization configured",
			args:    []string{"myproject"},
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
				deps.connFactory.EXPECT().Connection(tt.wantOrg).Return(nil, nil)
				deps.clientFact.EXPECT().Git(gomock.Any(), tt.wantOrg).Return(deps.gitClient, nil)
				deps.gitClient.EXPECT().GetRepositories(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, args git.GetRepositoriesArgs) (*[]git.GitRepository, error) {
						require.NotNil(t, args.Project)
						assert.Equal(t, "myproject", *args.Project)
						return sampleRepos(), nil
					})
				tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
				require.NoError(t, err)
				deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()
			}

			cmd := NewCmdRepoList(deps.cmd)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, deps.stdout.String(), "demo-repo")
		})
	}
}
