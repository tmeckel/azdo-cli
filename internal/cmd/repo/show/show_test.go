package show

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type dependencies struct {
	ctrl       *gomock.Controller
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	gitClient  *mocks.MockAzDOGitClient
	config     *mocks.MockConfig
	auth       *mocks.MockAuthConfig
	ios        *iostreams.IOStreams
	stdout     *bytes.Buffer
	stderr     *bytes.Buffer
}

var (
	ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	osc8Regexp = regexp.MustCompile(`\x1b]8;;[^\x1b]*\x1b\\`)
)

func cleanOutput(out *bytes.Buffer) string {
	cleaned := ansiRegexp.ReplaceAllString(out.String(), "")
	return osc8Regexp.ReplaceAllString(cleaned, "")
}

func newDependencies(t *testing.T) *dependencies {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, errOut := iostreams.Test()
	io.SetStdoutTTY(true)
	io.SetStderrTTY(true)

	deps := &dependencies{
		ctrl:       ctrl,
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		gitClient:  mocks.NewMockAzDOGitClient(ctrl),
		config:     mocks.NewMockConfig(ctrl),
		auth:       mocks.NewMockAuthConfig(ctrl),
		ios:        io,
		stdout:     out,
		stderr:     errOut,
	}

	deps.cmd.EXPECT().IOStreams().Return(deps.ios, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()

	return deps
}

func (d *dependencies) setupDefaultOrg(org string) {
	d.cmd.EXPECT().Config().Return(d.config, nil).AnyTimes()
	d.config.EXPECT().Authentication().Return(d.auth).AnyTimes()
	d.auth.EXPECT().GetDefaultOrganization().Return(org, nil).AnyTimes()
}

func sampleRepo() *git.GitRepository {
	projectID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	repoID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	parentID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	return &git.GitRepository{
		Id:            &repoID,
		Name:          types.ToPtr("demo-repo"),
		DefaultBranch: types.ToPtr("refs/heads/main"),
		RemoteUrl:     types.ToPtr("https://dev.azure.com/myorg/Fabrikam/_git/demo-repo"),
		SshUrl:        types.ToPtr("git@ssh.dev.azure.com:v3/myorg/Fabrikam/demo-repo"),
		WebUrl:        types.ToPtr("https://dev.azure.com/myorg/Fabrikam/_git/demo-repo"),
		Url:           types.ToPtr("https://dev.azure.com/myorg/_apis/git/repositories/22222222-2222-2222-2222-222222222222"),
		Project: &core.TeamProjectReference{
			Id:   &projectID,
			Name: types.ToPtr("Fabrikam"),
		},
		ParentRepository: &git.GitRepositoryRef{
			Id:   &parentID,
			Name: types.ToPtr("upstream-repo"),
		},
		IsDisabled:      types.ToPtr(false),
		IsFork:          types.ToPtr(true),
		IsInMaintenance: types.ToPtr(false),
		Size:            types.ToPtr(uint64(1258291)),
		ValidRemoteUrls: &[]string{
			"https://dev.azure.com/myorg/Fabrikam/_git/demo-repo",
		},
		Links: map[string]any{
			"web": map[string]any{
				"href": "https://dev.azure.com/myorg/Fabrikam/_git/demo-repo",
			},
		},
	}
}

func TestNewCmd_RegistersAsShowLeaf(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "show", cmd.Name())
	assert.Contains(t, cmd.Aliases, "view")
	assert.Contains(t, cmd.Aliases, "status")
	assert.True(t, strings.HasPrefix(cmd.Use, "show [ORGANIZATION/]PROJECT/REPO_ID_OR_NAME"))
	assert.NotNil(t, cmd.RunE)
	assert.NotNil(t, cmd.Flag("json"))
}

func TestNewCmd_RequiresOneArg(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.setupDefaultOrg("myorg")

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target argument is required")
}

func TestRunShow_TemplateOutput_BasicFields(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.clientFact.EXPECT().Git(gomock.Any(), "myorg").Return(deps.gitClient, nil)
	deps.gitClient.EXPECT().GetRepository(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args git.GetRepositoryArgs) (*git.GitRepository, error) {
			assert.Equal(t, "demo-repo", *args.RepositoryId)
			assert.Equal(t, "Fabrikam", *args.Project)
			return sampleRepo(), nil
		})

	opts := &showOptions{targetArg: "myorg/Fabrikam/demo-repo"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, "url:")
	assert.Contains(t, output, "https://dev.azure.com/myorg/_apis/git/repositories/22222222-2222-2222-2222-222222222222")
	assert.Contains(t, output, "id: 22222222-2222-2222-2222-222222222222")
	assert.Contains(t, output, "name: demo-repo")
	assert.Contains(t, output, "project: Fabrikam (11111111-1111-1111-1111-111111111111)")
	assert.Contains(t, output, "default branch: refs/heads/main")
	assert.Contains(t, output, "remote url: https://dev.azure.com/myorg/Fabrikam/_git/demo-repo")
	assert.Contains(t, output, "ssh url: git@ssh.dev.azure.com:v3/myorg/Fabrikam/demo-repo")
	assert.Contains(t, output, "is fork: true")
	assert.Contains(t, output, "parent: upstream-repo (33333333-3333-3333-3333-333333333333)")
	assert.Contains(t, output, "is disabled: false")
	assert.Contains(t, output, "is in maintenance: false")
	assert.Contains(t, output, "size: 1.2 MB")
}

func TestRunShow_TemplateOutput_OptionalFieldsOmitted(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.clientFact.EXPECT().Git(gomock.Any(), "myorg").Return(deps.gitClient, nil)
	repo := sampleRepo()
	repo.DefaultBranch = nil
	repo.ParentRepository = nil
	repo.IsFork = types.ToPtr(false)
	repo.IsDisabled = nil
	repo.IsInMaintenance = nil
	repo.Size = nil
	deps.gitClient.EXPECT().GetRepository(gomock.Any(), gomock.Any()).Return(repo, nil)

	opts := &showOptions{targetArg: "myorg/Fabrikam/demo-repo"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.NotContains(t, output, "default branch:")
	assert.NotContains(t, output, "parent:")
	assert.NotContains(t, output, "is disabled:")
	assert.NotContains(t, output, "is in maintenance:")
	assert.NotContains(t, output, "size:")
}

func TestRunShow_ProjectScopeParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		targetArg  string
		defaultOrg string
		wantOrg    string
		wantErr    string
	}{
		{
			name:      "explicit org",
			targetArg: "myorg/Fabrikam/demo-repo",
			wantOrg:   "myorg",
		},
		{
			name:       "implicit org from config",
			targetArg:  "Fabrikam/demo-repo",
			defaultOrg: "default-org",
			wantOrg:    "default-org",
		},
		{
			name:      "missing project segment",
			targetArg: "demo-repo",
			wantErr:   "invalid input",
		},
		{
			name:      "too many segments",
			targetArg: "org/proj/repo/extra",
			wantErr:   "invalid input",
		},
		{
			name:      "empty repo segment",
			targetArg: "org/Fabrikam/",
			wantErr:   "input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := newDependencies(t)
			if tt.defaultOrg != "" {
				deps.setupDefaultOrg(tt.defaultOrg)
			}
			if tt.wantErr == "" {
				deps.clientFact.EXPECT().Git(gomock.Any(), tt.wantOrg).Return(deps.gitClient, nil)
				deps.gitClient.EXPECT().GetRepository(gomock.Any(), gomock.Any()).Return(sampleRepo(), nil)
			}

			opts := &showOptions{targetArg: tt.targetArg}
			err := runShow(deps.cmd, opts)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRunShow_ErrorPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		targetArg  string
		setup      func(*dependencies) error
		wantErr    string
		wantErrIs  error
		defaultOrg string
	}{
		{
			name:      "client factory error",
			targetArg: "myorg/Fabrikam/demo-repo",
			setup: func(deps *dependencies) error {
				expectedErr := errors.New("connection failed")
				deps.clientFact.EXPECT().Git(gomock.Any(), "myorg").Return(nil, expectedErr)
				return expectedErr
			},
			wantErrIs: errors.New("connection failed"),
		},
		{
			name:      "sdk error",
			targetArg: "myorg/Fabrikam/demo-repo",
			setup: func(deps *dependencies) error {
				expectedErr := errors.New("API error")
				deps.clientFact.EXPECT().Git(gomock.Any(), "myorg").Return(deps.gitClient, nil)
				deps.gitClient.EXPECT().GetRepository(gomock.Any(), gomock.Any()).Return(nil, expectedErr)
				return expectedErr
			},
			wantErrIs: errors.New("API error"),
		},
		{
			name:      "nil repo",
			targetArg: "myorg/Fabrikam/demo-repo",
			setup: func(deps *dependencies) error {
				deps.clientFact.EXPECT().Git(gomock.Any(), "myorg").Return(deps.gitClient, nil)
				deps.gitClient.EXPECT().GetRepository(gomock.Any(), gomock.Any()).Return(nil, nil)
				return nil
			},
			wantErr: `repository "demo-repo" not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := newDependencies(t)
			if tt.defaultOrg != "" {
				deps.setupDefaultOrg(tt.defaultOrg)
			}
			expectedErr := tt.setup(deps)

			err := runShow(deps.cmd, &showOptions{targetArg: tt.targetArg})
			require.Error(t, err)
			if tt.wantErr != "" {
				assert.Contains(t, err.Error(), tt.wantErr)
			}
			if expectedErr != nil {
				assert.ErrorIs(t, err, expectedErr)
			}
		})
	}
}

func TestRunShow_JSONOutput(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.clientFact.EXPECT().Git(gomock.Any(), "myorg").Return(deps.gitClient, nil)
	deps.gitClient.EXPECT().GetRepository(gomock.Any(), gomock.Any()).Return(sampleRepo(), nil)

	opts := &showOptions{
		targetArg: "myorg/Fabrikam/demo-repo",
		exporter:  util.NewJSONExporter(),
	}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, `"id":"22222222-2222-2222-2222-222222222222"`)
	assert.Contains(t, output, `"name":"demo-repo"`)
	assert.Contains(t, output, `"defaultBranch":"refs/heads/main"`)
	assert.Contains(t, output, `"project"`)
	assert.Contains(t, output, `"parentRepository"`)
	assert.NotContains(t, output, `"templateData"`)
}
