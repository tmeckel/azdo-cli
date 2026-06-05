package show

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type fakeShowDeps struct {
	ctrl       *gomock.Controller
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	taskClient *mocks.MockTaskAgentClient
	config     *mocks.MockConfig
	auth       *mocks.MockAuthConfig
	ios        *iostreams.IOStreams
	stdout     *bytes.Buffer
}

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func cleanOutput(out *bytes.Buffer) string {
	return ansiRegexp.ReplaceAllString(out.String(), "")
}

func setupFakeDeps(t *testing.T) *fakeShowDeps {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(true)
	io.SetStderrTTY(true)

	deps := &fakeShowDeps{
		ctrl:       ctrl,
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		taskClient: mocks.NewMockTaskAgentClient(ctrl),
		config:     mocks.NewMockConfig(ctrl),
		auth:       mocks.NewMockAuthConfig(ctrl),
		ios:        io,
		stdout:     out,
	}

	deps.cmd.EXPECT().IOStreams().Return(deps.ios, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()

	return deps
}

func (d *fakeShowDeps) setupDefaultOrg(org string) {
	d.cmd.EXPECT().Config().Return(d.config, nil).AnyTimes()
	d.config.EXPECT().Authentication().Return(d.auth).AnyTimes()
	d.auth.EXPECT().GetDefaultOrganization().Return(org, nil).AnyTimes()
}

func samplePool() *taskagent.TaskAgentPool {
	return &taskagent.TaskAgentPool{
		Id:            types.ToPtr(7),
		Name:          types.ToPtr("Default"),
		PoolType:      &taskagent.TaskAgentPoolTypeValues.Automation,
		IsHosted:      types.ToPtr(true),
		IsLegacy:      types.ToPtr(false),
		Size:          types.ToPtr(3),
		AutoProvision: types.ToPtr(true),
		AutoUpdate:    types.ToPtr(true),
		Scope:         types.ToPtr(uuid.MustParse("a1b2c3d4-e5f6-7890-abcd-ef1234567890")),
		CreatedOn:     &azuredevops.Time{Time: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)},
		CreatedBy: &webapi.IdentityRef{
			DisplayName: types.ToPtr("Alice"),
			UniqueName:  types.ToPtr("alice@contoso.com"),
		},
		Owner: &webapi.IdentityRef{
			DisplayName: types.ToPtr("Bob"),
			UniqueName:  types.ToPtr("bob@contoso.com"),
		},
	}
}

func TestNewCmd_RegistersAsShowLeaf(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "show", cmd.Name())
	assert.Contains(t, cmd.Aliases, "view")
	assert.Contains(t, cmd.Aliases, "status")
	assert.True(t, strings.HasPrefix(cmd.Use, "show [ORGANIZATION/]POOL"))
	assert.NotNil(t, cmd.RunE)
}

func TestNewCmd_RequiresOneArg(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.setupDefaultOrg("myorg")

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pool target is required")
}

func TestRunShow_ResolveByPositiveInteger(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	// For numeric "7", ResolvePool uses Atoi directly; no GetAgentPools call.
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args taskagent.GetAgentPoolArgs) (*taskagent.TaskAgentPool, error) {
			assert.Equal(t, 7, *args.PoolId)
			return samplePool(), nil
		})

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)
}

func TestRunShow_ResolveByName(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	poolList := []taskagent.TaskAgentPool{
		{Id: types.ToPtr(7), Name: types.ToPtr("Default")},
	}
	deps.taskClient.EXPECT().GetAgentPools(gomock.Any(), gomock.Any()).
		Return(&poolList, nil)
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args taskagent.GetAgentPoolArgs) (*taskagent.TaskAgentPool, error) {
			assert.Equal(t, 7, *args.PoolId)
			return samplePool(), nil
		})

	opts := &showOptions{targetArg: "myorg/Default"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)
}

func TestRunShow_BasicCall(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(samplePool(), nil)

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, "id: 7")
	assert.Contains(t, output, "name: Default")
}

func TestRunShow_OrgFromConfigDefault(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.setupDefaultOrg("myorg")
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(samplePool(), nil)

	opts := &showOptions{targetArg: "7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, "id: 7")
}

func TestRunShow_OrgFromPositional(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "otherorg").Return(deps.taskClient, nil)
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(samplePool(), nil)

	opts := &showOptions{targetArg: "otherorg/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, "id: 7")
}

func TestRunShow_TemplateOutput_BasicFields(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(samplePool(), nil)

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, "id: 7")
	assert.Contains(t, output, "name: Default")
	assert.Contains(t, output, "size: 3")
	assert.Contains(t, output, "created by: Alice (alice@contoso.com)")
	assert.Contains(t, output, "owner: Bob (bob@contoso.com)")
}

func TestRunShow_TemplateOutput_NoCreatedBy(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	pool := samplePool()
	pool.CreatedBy = nil
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(pool, nil)

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.NotContains(t, output, "created by:")
}

func TestRunShow_TemplateOutput_NoOwner(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	pool := samplePool()
	pool.Owner = nil
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(pool, nil)

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.NotContains(t, output, "owner:")
}

func TestRunShow_TemplateOutput_AutoUpdateTrue(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	pool := samplePool()
	pool.AutoUpdate = types.ToPtr(true)
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(pool, nil)

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, "auto update: true")
}

func TestRunShow_TemplateOutput_AutoUpdateFalse(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	pool := samplePool()
	pool.AutoUpdate = types.ToPtr(false)
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(pool, nil)

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, "auto update: false")
}

func TestRunShow_TemplateOutput_AutoProvisionTrue(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	pool := samplePool()
	pool.AutoProvision = types.ToPtr(true)
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(pool, nil)

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, "auto provision: true")
}

func TestRunShow_TemplateOutput_IsHosted(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	pool := samplePool()
	pool.IsHosted = types.ToPtr(true)
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(pool, nil)

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, "is hosted: true")
}

func TestRunShow_TemplateOutput_IsLegacy(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	pool := samplePool()
	pool.IsLegacy = types.ToPtr(true)
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(pool, nil)

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, "is legacy: true")
}

func TestRunShow_TemplateOutput_CreatedOn(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(samplePool(), nil)

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, "created on:")
	assert.Contains(t, output, "2024-01-15")
}

func TestRunShow_JSONOutput(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(samplePool(), nil)

	exporter := util.NewJSONExporter()
	opts := &showOptions{targetArg: "myorg/7", exporter: exporter}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, `"id"`)
	assert.Contains(t, output, `"name"`)
	assert.Contains(t, output, `"poolType"`)
	assert.Contains(t, output, `"isHosted"`)
	assert.Contains(t, output, `"createdOn"`)
	assert.Contains(t, output, `"createdBy"`)
	assert.Contains(t, output, `7`)
	assert.Contains(t, output, `"Default"`)
	assert.True(t, strings.HasPrefix(strings.TrimSpace(output), "{"))
}

func TestRunShow_RawFlag(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	pool := samplePool()
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(pool, nil)

	opts := &showOptions{targetArg: "myorg/7", raw: true}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)
}

func TestRunShow_ClientFactoryError(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	expectedErr := fmt.Errorf("connection failed")
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(nil, expectedErr)

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestRunShow_SDKError(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)
	expectedErr := fmt.Errorf("API error")
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestRunShow_PoolNotFound(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(nil, nil)

	opts := &showOptions{targetArg: "myorg/999"}
	err := runShow(deps.cmd, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRunShow_InvalidTarget(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	deps.taskClient.EXPECT().GetAgentPools(gomock.Any(), gomock.Any()).
		Return(&[]taskagent.TaskAgentPool{}, nil)

	opts := &showOptions{targetArg: "myorg/NonExistentPool"}
	err := runShow(deps.cmd, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRunShow_PoolTypeAutomation(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	pool := samplePool()
	pool.PoolType = &taskagent.TaskAgentPoolTypeValues.Automation
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(pool, nil)

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, "automation")
}

func TestRunShow_PoolTypeDeployment(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	pool := samplePool()
	pool.PoolType = &taskagent.TaskAgentPoolTypeValues.Deployment
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(pool, nil)

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, "deployment")
}

func TestNewCmd_HasFlags(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	rawFlag := cmd.Flag("raw")
	require.NotNil(t, rawFlag)
	assert.Equal(t, "r", rawFlag.Shorthand)

	jsonFlag := cmd.Flag("json")
	require.NotNil(t, jsonFlag)
}

func TestRunShow_ParentCommandWiring(t *testing.T) {
	t.Parallel()

	// Verify the cobra path: pipelines pool show
	rootCmd := &cobra.Command{Use: "root"}
	poolCmd := &cobra.Command{Use: "pool"}
	showCmd := NewCmd(nil)
	poolCmd.AddCommand(showCmd)
	rootCmd.AddCommand(poolCmd)

	found, _, err := rootCmd.Find([]string{"pool", "show", "myorg/7"})
	require.NoError(t, err)
	assert.Equal(t, "show", found.Name())
}

func TestRunShow_TemplateOutput_Scope(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(samplePool(), nil)

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, "scope:")
	assert.NotContains(t, output, "scope: \n")
}

func TestRunShow_TemplateOutput_ScopeNil(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	pool := samplePool()
	pool.Scope = nil
	deps.taskClient.EXPECT().GetAgentPool(gomock.Any(), gomock.Any()).Return(pool, nil)

	opts := &showOptions{targetArg: "myorg/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.NotContains(t, output, "scope:")
}
