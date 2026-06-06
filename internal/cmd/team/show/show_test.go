package show

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

type fakeShowDeps struct {
	cmd     *mocks.MockCmdContext
	cfg     *mocks.MockConfig
	authCfg *mocks.MockAuthConfig
	clientF *mocks.MockClientFactory
	core    *mocks.MockCoreClient
	stdout  *bytes.Buffer
	ctrl    *gomock.Controller
}

func setupFakeDeps(t *testing.T, organization string) *fakeShowDeps {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &fakeShowDeps{
		cmd:     mocks.NewMockCmdContext(ctrl),
		cfg:     mocks.NewMockConfig(ctrl),
		authCfg: mocks.NewMockAuthConfig(ctrl),
		clientF: mocks.NewMockClientFactory(ctrl),
		core:    mocks.NewMockCoreClient(ctrl),
		stdout:  out,
		ctrl:    ctrl,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientF).AnyTimes()
	deps.cmd.EXPECT().Config().Return(deps.cfg, nil).AnyTimes()
	deps.cfg.EXPECT().Authentication().Return(deps.authCfg).AnyTimes()
	deps.authCfg.EXPECT().GetDefaultOrganization().Return(organization, nil).AnyTimes()

	return deps
}

func TestShow_MissingTeamArg(t *testing.T) {
	deps := setupFakeDeps(t, "defaultOrg")

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, "team argument required")
}

func TestShow_TargetArg_ParsesOrgSlashProjectSlashTeam(t *testing.T) {
	deps := setupFakeDeps(t, "defaultOrg")

	teamUUID := uuid.MustParse("00000001-0000-0000-0000-000000000001")
	deps.clientF.EXPECT().Core(gomock.Any(), "myOrg").Return(deps.core, nil)
	deps.core.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetTeamArgs) (*core.WebApiTeam, error) {
			assert.Equal(t, "myProject", *args.ProjectId)
			assert.Equal(t, "My Team", *args.TeamId)
			return &core.WebApiTeam{
				Id:   &teamUUID,
				Name: types.ToPtr("My Team"),
			}, nil
		})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/My Team"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestShow_DefaultsToConfiguredOrganization(t *testing.T) {
	deps := setupFakeDeps(t, "defaultOrg")

	teamUUID := uuid.MustParse("00000001-0000-0000-0000-000000000001")
	deps.clientF.EXPECT().Core(gomock.Any(), "defaultOrg").Return(deps.core, nil)
	deps.core.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetTeamArgs) (*core.WebApiTeam, error) {
			assert.Equal(t, "myProject", *args.ProjectId)
			assert.Equal(t, "My Team", *args.TeamId)
			return &core.WebApiTeam{
				Id:   &teamUUID,
				Name: types.ToPtr("My Team"),
			}, nil
		})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myProject/My Team"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestShow_TeamByGUID(t *testing.T) {
	deps := setupFakeDeps(t, "myOrg")

	teamUUID := uuid.MustParse("00000002-0000-0000-0000-000000000000")
	deps.clientF.EXPECT().Core(gomock.Any(), "myOrg").Return(deps.core, nil)
	deps.core.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetTeamArgs) (*core.WebApiTeam, error) {
			assert.Equal(t, "myProject", *args.ProjectId)
			assert.Equal(t, "00000002-0000-0000-0000-000000000000", *args.TeamId)
			return &core.WebApiTeam{
				Id:   &teamUUID,
				Name: types.ToPtr("My Team"),
			}, nil
		})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/00000002-0000-0000-0000-000000000000"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestShow_JSONOutput(t *testing.T) {
	deps := setupFakeDeps(t, "myOrg")

	teamUUID := uuid.MustParse("00000001-0000-0000-0000-000000000001")
	deps.clientF.EXPECT().Core(gomock.Any(), "myOrg").Return(deps.core, nil)
	deps.core.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(&core.WebApiTeam{
			Id:          &teamUUID,
			Name:        types.ToPtr("My Team"),
			Description: types.ToPtr("A test team"),
			Url:         types.ToPtr("https://dev.azure.com/myOrg/_apis/teams/00000001-0000-0000-0000-000000000001"),
			ProjectName: types.ToPtr("myProject"),
			ProjectId:   &teamUUID,
		}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/My Team", "--json"})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, deps.stdout.String(), `"id"`)
	assert.Contains(t, deps.stdout.String(), `"name"`)
	assert.Contains(t, deps.stdout.String(), `"description"`)
	assert.Contains(t, deps.stdout.String(), `"My Team"`)
}

func TestShow_TemplateOutput_BasicFields(t *testing.T) {
	deps := setupFakeDeps(t, "myOrg")

	teamUUID := uuid.MustParse("00000001-0000-0000-0000-000000000001")
	id := &identity.Identity{
		ProviderDisplayName: types.ToPtr("John Doe"),
		Descriptor:          types.ToPtr("john.doe@example.com"),
	}
	deps.clientF.EXPECT().Core(gomock.Any(), "myOrg").Return(deps.core, nil)
	deps.core.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(&core.WebApiTeam{
			Id:          &teamUUID,
			Name:        types.ToPtr("My Team"),
			Description: types.ToPtr("A test team"),
			Url:         types.ToPtr("https://dev.azure.com/myOrg/_apis/teams/00000001-0000-0000-0000-000000000001"),
			ProjectName: types.ToPtr("myProject"),
			ProjectId:   &teamUUID,
			Identity:    id,
		}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/My Team"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := deps.stdout.String()
	assert.Contains(t, output, "My Team")
	assert.Contains(t, output, "A test team")
	assert.Contains(t, output, "myProject")
	assert.Contains(t, output, "John Doe")
	assert.Contains(t, output, "john.doe@example.com")

	lines := splitLines(output)
	assert.GreaterOrEqual(t, len(lines), 6, "expected at least 6 non-empty lines for 6 populated fields")
}

func TestShow_TemplateOutput_OnlyPresentFields(t *testing.T) {
	deps := setupFakeDeps(t, "myOrg")

	teamUUID := uuid.MustParse("00000001-0000-0000-0000-000000000001")
	deps.clientF.EXPECT().Core(gomock.Any(), "myOrg").Return(deps.core, nil)
	deps.core.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(&core.WebApiTeam{
			Id:   &teamUUID,
			Name: types.ToPtr("Minimal Team"),
			Url:  types.ToPtr("https://dev.azure.com/myOrg/_apis/teams/00000001-0000-0000-0000-000000000001"),
		}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/Minimal Team"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := deps.stdout.String()
	assert.Contains(t, output, "Minimal Team")
	assert.NotContains(t, output, "description:")
	assert.NotContains(t, output, "project:")
	assert.NotContains(t, output, "identity:")

	lines := splitLines(output)
	assert.GreaterOrEqual(t, len(lines), 3, "expected at least 3 non-empty lines for 3 populated fields")
	for _, line := range lines {
		assert.True(t, strings.Contains(line, "url:") || strings.Contains(line, "id:") || strings.Contains(line, "name:"),
			"unexpected label line: %q", line)
	}
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func TestShow_PropagatesSDKError(t *testing.T) {
	deps := setupFakeDeps(t, "myOrg")

	deps.clientF.EXPECT().Core(gomock.Any(), "myOrg").Return(deps.core, nil)
	deps.core.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(nil, assert.AnError)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/My Team"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to get team")
}
