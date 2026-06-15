package list

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type fakeListDeps struct {
	ctrl       *gomock.Controller
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	taskClient *mocks.MockTaskAgentClient
	config     *mocks.MockConfig
	auth       *mocks.MockAuthConfig
	stdout     *bytes.Buffer
}

func setupFakeDeps(t *testing.T, organization string) *fakeListDeps {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &fakeListDeps{
		ctrl:       ctrl,
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		taskClient: mocks.NewMockTaskAgentClient(ctrl),
		stdout:     out,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	if organization != "" {
		deps.clientFact.EXPECT().TaskAgent(gomock.Any(), organization).Return(deps.taskClient, nil).AnyTimes()
	}

	return deps
}

func (d *fakeListDeps) setupDefaultOrg(org string) {
	d.config = mocks.NewMockConfig(d.ctrl)
	d.auth = mocks.NewMockAuthConfig(d.ctrl)
	d.cmd.EXPECT().Config().Return(d.config, nil).AnyTimes()
	d.config.EXPECT().Authentication().Return(d.auth).AnyTimes()
	d.auth.EXPECT().GetDefaultOrganization().Return(org, nil).AnyTimes()
}

func samplePool(id int, name string, poolType taskagent.TaskAgentPoolType) taskagent.TaskAgentPool {
	scope := uuid.MustParse("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	createdOn := azuredevops.Time{Time: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)}
	return taskagent.TaskAgentPool{
		Id:            types.ToPtr(id),
		Name:          types.ToPtr(name),
		PoolType:      &poolType,
		Scope:         &scope,
		Size:          types.ToPtr(3),
		IsHosted:      types.ToPtr(true),
		IsLegacy:      types.ToPtr(false),
		AutoProvision: types.ToPtr(true),
		AutoUpdate:    types.ToPtr(true),
		CreatedOn:     &createdOn,
		CreatedBy: &webapi.IdentityRef{
			Id:          types.ToPtr("creator-id"),
			DisplayName: types.ToPtr("Alice"),
			UniqueName:  types.ToPtr("alice@contoso.com"),
		},
	}
}

func TestNewCmd_RegistersAsListLeaf(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "list [ORGANIZATION]", cmd.Use)
	assert.ElementsMatch(t, []string{"ls", "l"}, cmd.Aliases)
	assert.NotNil(t, cmd.RunE)
}

func TestNewCmd_RequiresAtMostOneArg(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	require.NoError(t, cmd.Args(cmd, []string{}))
	require.NoError(t, cmd.Args(cmd, []string{"myorg"}))
	require.Error(t, cmd.Args(cmd, []string{"myorg", "extra"}))
}

func TestNewCmd_HasFlags(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	f := cmd.Flags()

	require.NotNil(t, f.Lookup("name"))
	require.NotNil(t, f.Lookup("pool-type"))
	require.NotNil(t, f.Lookup("max-items"))
	assert.NotNil(t, f.Lookup("json"))
	assert.NotNil(t, f.Lookup("jq"))
	assert.NotNil(t, f.Lookup("template"))
}

func TestNewCmd_RejectsInvalidPoolType(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	cmd.SetArgs([]string{"--pool-type", "invalid"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "valid values are {automation|deployment}")
}

func TestRun_BasicCall(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "myorg")
	pools := []taskagent.TaskAgentPool{samplePool(7, "Default", taskagent.TaskAgentPoolTypeValues.Automation)}
	deps.taskClient.EXPECT().GetAgentPools(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args taskagent.GetAgentPoolsArgs) (*[]taskagent.TaskAgentPool, error) {
			assert.Nil(t, args.PoolName)
			assert.Nil(t, args.PoolType)
			return &pools, nil
		},
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = run(deps.cmd, &opts{orgArg: "myorg"})
	require.NoError(t, err)

	output := deps.stdout.String()
	assert.Contains(t, output, "7\tDefault\tautomation\ta1b2c3d4-e5f6-7890-abcd-ef1234567890\t3\ttrue\tfalse\ttrue\t2024-01-15")
	assert.Contains(t, output, "7")
	assert.Contains(t, output, "Default")
	assert.Contains(t, output, "automation")
	assert.Contains(t, output, "a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	assert.Contains(t, output, "3")
	assert.Contains(t, output, "true")
	assert.Contains(t, output, "false")
	assert.Contains(t, output, "2024-01-15")
}

func TestRun_OrgFromArg(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "explicit-org")
	pools := []taskagent.TaskAgentPool{samplePool(1, "Default", taskagent.TaskAgentPoolTypeValues.Automation)}
	deps.taskClient.EXPECT().GetAgentPools(gomock.Any(), gomock.Any()).Return(&pools, nil)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = run(deps.cmd, &opts{orgArg: "explicit-org"})
	require.NoError(t, err)
	assert.Contains(t, deps.stdout.String(), "Default")
}

func TestRun_OrgFromDefaultConfig(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "")
	deps.setupDefaultOrg("default-org")
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "default-org").Return(deps.taskClient, nil).AnyTimes()
	pools := []taskagent.TaskAgentPool{samplePool(1, "Default", taskagent.TaskAgentPoolTypeValues.Automation)}
	deps.taskClient.EXPECT().GetAgentPools(gomock.Any(), gomock.Any()).Return(&pools, nil)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = run(deps.cmd, &opts{})
	require.NoError(t, err)
	assert.Contains(t, deps.stdout.String(), "Default")
}

func TestRun_ProjectScopeRejected(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "")
	err := run(deps.cmd, &opts{orgArg: "org/project"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project scope not allowed")
}

func TestRun_NoDefaultOrg(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "")
	deps.setupDefaultOrg("")

	err := run(deps.cmd, &opts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no organization specified")
}

func TestRun_InvalidMaxItemsNegative(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	err := run(deps.cmd, &opts{orgArg: "org", maxItems: -5})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --max-items")
}

func TestRun_ClientFactoryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	cmd := mocks.NewMockCmdContext(ctrl)
	clientFact := mocks.NewMockClientFactory(ctrl)
	cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmd.EXPECT().ClientFactory().Return(clientFact).AnyTimes()

	cfg := mocks.NewMockConfig(ctrl)
	auth := mocks.NewMockAuthConfig(ctrl)
	cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(auth).AnyTimes()
	auth.EXPECT().GetDefaultOrganization().Return("myorg", nil)

	clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(nil, fmt.Errorf("connection failed"))

	err := run(cmd, &opts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection failed")
}

func TestRun_SDKError(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	deps.taskClient.EXPECT().GetAgentPools(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("API error"))

	err := run(deps.cmd, &opts{orgArg: "org"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
}

func TestRun_FilterByName(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	pools := []taskagent.TaskAgentPool{samplePool(1, "Default", taskagent.TaskAgentPoolTypeValues.Automation)}
	deps.taskClient.EXPECT().GetAgentPools(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args taskagent.GetAgentPoolsArgs) (*[]taskagent.TaskAgentPool, error) {
			require.NotNil(t, args.PoolName)
			assert.Equal(t, "Default", *args.PoolName)
			return &pools, nil
		},
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = run(deps.cmd, &opts{orgArg: "org", name: "Default"})
	require.NoError(t, err)
}

func TestRun_PoolTypeFilter(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	pools := []taskagent.TaskAgentPool{samplePool(1, "Default", taskagent.TaskAgentPoolTypeValues.Deployment)}
	deps.taskClient.EXPECT().GetAgentPools(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args taskagent.GetAgentPoolsArgs) (*[]taskagent.TaskAgentPool, error) {
			require.NotNil(t, args.PoolType)
			assert.Equal(t, taskagent.TaskAgentPoolType("deployment"), *args.PoolType)
			return &pools, nil
		},
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = run(deps.cmd, &opts{orgArg: "org", poolType: "Deployment"})
	require.NoError(t, err)
}

func TestRun_MaxItemsCaps(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	pools := []taskagent.TaskAgentPool{
		samplePool(1, "pool-1", taskagent.TaskAgentPoolTypeValues.Automation),
		samplePool(2, "pool-2", taskagent.TaskAgentPoolTypeValues.Automation),
	}
	deps.taskClient.EXPECT().GetAgentPools(gomock.Any(), gomock.Any()).Return(&pools, nil)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = run(deps.cmd, &opts{orgArg: "org", maxItems: 1})
	require.NoError(t, err)

	output := deps.stdout.String()
	assert.Contains(t, output, "pool-1")
	assert.NotContains(t, output, "pool-2")
}

func TestRun_JSONOutput(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	pools := []taskagent.TaskAgentPool{samplePool(7, "Default", taskagent.TaskAgentPoolTypeValues.Automation)}
	deps.taskClient.EXPECT().GetAgentPools(gomock.Any(), gomock.Any()).Return(&pools, nil)

	exporter := util.NewJSONExporter()
	err := run(deps.cmd, &opts{orgArg: "org", exporter: exporter})
	require.NoError(t, err)

	var parsed []map[string]any
	err = json.Unmarshal(deps.stdout.Bytes(), &parsed)
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	assert.Equal(t, float64(7), parsed[0]["id"])
	assert.Equal(t, "Default", parsed[0]["name"])
	assert.Equal(t, "automation", parsed[0]["poolType"])
	assert.NotContains(t, parsed[0], "type")
	assert.Equal(t, "a1b2c3d4-e5f6-7890-abcd-ef1234567890", parsed[0]["scope"])
	assert.Equal(t, float64(3), parsed[0]["size"])
	assert.Equal(t, true, parsed[0]["isHosted"])
	assert.Equal(t, false, parsed[0]["isLegacy"])
	assert.Equal(t, true, parsed[0]["autoProvision"])
	assert.Equal(t, true, parsed[0]["autoUpdate"])
	assert.Contains(t, parsed[0]["createdOn"], "2024-01-15")
	createdBy, ok := parsed[0]["createdBy"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "creator-id", createdBy["id"])
	assert.Equal(t, "Alice", createdBy["displayName"])
	assert.Equal(t, "alice@contoso.com", createdBy["uniqueName"])
}

func TestRun_JSONOutputEmpty(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	pools := []taskagent.TaskAgentPool{}
	deps.taskClient.EXPECT().GetAgentPools(gomock.Any(), gomock.Any()).Return(&pools, nil)

	exporter := util.NewJSONExporter()
	err := run(deps.cmd, &opts{orgArg: "org", exporter: exporter})
	require.NoError(t, err)

	assert.Equal(t, "[]\n", deps.stdout.String())
}

func TestRun_NullResponseIsEmpty(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	deps.taskClient.EXPECT().GetAgentPools(gomock.Any(), gomock.Any()).Return(nil, nil)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = run(deps.cmd, &opts{orgArg: "org"})
	require.NoError(t, err)
	assert.Equal(t, "", deps.stdout.String())
}
