package list

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
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
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	tac        *mocks.MockTaskAgentClient
	stdout     *bytes.Buffer
	stderr     *bytes.Buffer
}

func setupFakeDeps(t *testing.T, organization string) *fakeListDeps {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, errOut := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &fakeListDeps{
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		tac:        mocks.NewMockTaskAgentClient(ctrl),
		stdout:     out,
		stderr:     errOut,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), organization).Return(deps.tac, nil).AnyTimes()

	return deps
}

func sampleAgent(id int, name string, status taskagent.TaskAgentStatus, enabled bool) taskagent.TaskAgent {
	createdOn := azuredevops.Time{Time: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)}
	return taskagent.TaskAgent{
		Id:             types.ToPtr(id),
		Name:           types.ToPtr(name),
		Status:         &status,
		Enabled:        types.ToPtr(enabled),
		Version:        types.ToPtr("4.240.0"),
		OsDescription:  types.ToPtr("Linux 5.15.0-1050-azure"),
		MaxParallelism: types.ToPtr(1),
		CreatedOn:      &createdOn,
	}
}

func TestNewCmd_RegistersAsListLeaf(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "list [ORGANIZATION/]POOL", cmd.Use)
	assert.ElementsMatch(t, []string{"ls", "l"}, cmd.Aliases)
	assert.NotNil(t, cmd.RunE)
}

func TestNewCmd_RequiresExactlyOneArg(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	err := cmd.Args(cmd, []string{})
	require.Error(t, err)
	err = cmd.Args(cmd, []string{"org", "extra"})
	require.Error(t, err)
}

func TestNewCmd_HasFlags(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	f := cmd.Flags()

	assert.Nil(t, f.Lookup("pool-id"))
	require.NotNil(t, f.Lookup("filter"))
	require.NotNil(t, f.Lookup("include-capabilities"))
	require.NotNil(t, f.Lookup("max-items"))
	assert.NotNil(t, f.Lookup("json"))
	assert.NotNil(t, f.Lookup("jq"))
	assert.NotNil(t, f.Lookup("template"))
}

func TestRunList_BasicCall(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "myorg")
	agents := []taskagent.TaskAgent{
		sampleAgent(7, "agent-01", taskagent.TaskAgentStatusValues.Online, true),
		sampleAgent(8, "agent-02", taskagent.TaskAgentStatusValues.Offline, true),
	}
	deps.tac.EXPECT().GetAgentPools(gomock.Any(), gomock.Any()).Return(&[]taskagent.TaskAgentPool{{Id: types.ToPtr(1), Name: types.ToPtr("1")}}, nil).AnyTimes()
	deps.tac.EXPECT().GetAgents(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args taskagent.GetAgentsArgs) (*[]taskagent.TaskAgent, error) {
			require.NotNil(t, args.PoolId)
			assert.Equal(t, 1, *args.PoolId)
			return &agents, nil
		},
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = run(deps.cmd, &opts{targetArg: "myorg/1"})
	require.NoError(t, err)

	output := deps.stdout.String()
	assert.Contains(t, output, "7")
	assert.Contains(t, output, "agent-01")
	assert.Contains(t, output, "8")
	assert.Contains(t, output, "agent-02")
	assert.Contains(t, output, "online")
	assert.Contains(t, output, "offline")
	assert.Contains(t, output, "true")
	assert.Contains(t, output, "4.240.0")
	assert.Contains(t, output, "Linux")
}

func TestRunList_OrgFromArg(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "explicit-org")
	agents := []taskagent.TaskAgent{sampleAgent(1, "agent-01", taskagent.TaskAgentStatusValues.Online, true)}
	deps.tac.EXPECT().GetAgentPools(gomock.Any(), gomock.Any()).Return(&[]taskagent.TaskAgentPool{{Id: types.ToPtr(1), Name: types.ToPtr("1")}}, nil).AnyTimes()
	deps.tac.EXPECT().GetAgents(gomock.Any(), gomock.Any()).Return(&agents, nil)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = run(deps.cmd, &opts{targetArg: "explicit-org/1"})
	require.NoError(t, err)
	assert.Contains(t, deps.stdout.String(), "agent-01")
}

func TestRunList_ProjectScopeRejected(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	err := run(deps.cmd, &opts{targetArg: "org/proj/1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not accept a project scope")
}

func TestRunList_NoDefaultOrg(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	cmd := mocks.NewMockCmdContext(ctrl)
	cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmd.EXPECT().ClientFactory().Return(mocks.NewMockClientFactory(ctrl)).AnyTimes()

	cfg := mocks.NewMockConfig(ctrl)
	auth := mocks.NewMockAuthConfig(ctrl)
	cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(auth).AnyTimes()
	auth.EXPECT().GetDefaultOrganization().Return("", fmt.Errorf("no default org"))

	err := run(cmd, &opts{targetArg: "1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no organization specified")
}

func TestRunList_InvalidMaxItemsNegative(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	err := run(deps.cmd, &opts{targetArg: "org/1", maxItems: -5})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --max-items")
}

func TestRunList_ClientFactoryError(t *testing.T) {
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

	err := run(cmd, &opts{targetArg: "1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection failed")
}

func TestRunList_SDKError(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	deps.tac.EXPECT().GetAgents(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("API error"))

	err := run(deps.cmd, &opts{targetArg: "org/1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
}

func TestRunList_EmptyResult(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	agents := []taskagent.TaskAgent{}
	deps.tac.EXPECT().GetAgents(gomock.Any(), gomock.Any()).Return(&agents, nil)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = run(deps.cmd, &opts{targetArg: "org/1"})
	require.NoError(t, err)
}

func TestRunList_FilterByName(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	agents := []taskagent.TaskAgent{sampleAgent(1, "filtered-agent", taskagent.TaskAgentStatusValues.Online, true)}
	deps.tac.EXPECT().GetAgents(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args taskagent.GetAgentsArgs) (*[]taskagent.TaskAgent, error) {
			require.NotNil(t, args.AgentName)
			assert.Equal(t, "filtered-agent", *args.AgentName)
			return &agents, nil
		},
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = run(deps.cmd, &opts{targetArg: "org/1", filter: "filtered-agent"})
	require.NoError(t, err)
}

func TestRunList_IncludeCapabilities(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	agents := []taskagent.TaskAgent{sampleAgent(1, "agent-01", taskagent.TaskAgentStatusValues.Online, true)}
	deps.tac.EXPECT().GetAgents(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args taskagent.GetAgentsArgs) (*[]taskagent.TaskAgent, error) {
			require.NotNil(t, args.IncludeCapabilities)
			assert.True(t, *args.IncludeCapabilities)
			return &agents, nil
		},
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = run(deps.cmd, &opts{targetArg: "org/1", includeCapabilities: true})
	require.NoError(t, err)
}

func TestRunList_MaxItemsCaps(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	agents := []taskagent.TaskAgent{
		sampleAgent(1, "a1", taskagent.TaskAgentStatusValues.Online, true),
		sampleAgent(2, "a2", taskagent.TaskAgentStatusValues.Online, true),
		sampleAgent(3, "a3", taskagent.TaskAgentStatusValues.Online, true),
	}
	deps.tac.EXPECT().GetAgents(gomock.Any(), gomock.Any()).Return(&agents, nil)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = run(deps.cmd, &opts{targetArg: "org/1", maxItems: 1})
	require.NoError(t, err)

	output := deps.stdout.String()
	assert.Contains(t, output, "a1")
	assert.NotContains(t, output, "a2")
	assert.NotContains(t, output, "a3")
}

func TestRunList_MaxItemsExceedsResult(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	agents := []taskagent.TaskAgent{
		sampleAgent(1, "a1", taskagent.TaskAgentStatusValues.Online, true),
		sampleAgent(2, "a2", taskagent.TaskAgentStatusValues.Online, true),
	}
	deps.tac.EXPECT().GetAgents(gomock.Any(), gomock.Any()).Return(&agents, nil)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = run(deps.cmd, &opts{targetArg: "org/1", maxItems: 100})
	require.NoError(t, err)

	output := deps.stdout.String()
	assert.Contains(t, output, "a1")
	assert.Contains(t, output, "a2")
}

func TestRunList_JSONOutput(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	agents := []taskagent.TaskAgent{sampleAgent(7, "agent-01", taskagent.TaskAgentStatusValues.Online, true)}
	deps.tac.EXPECT().GetAgents(gomock.Any(), gomock.Any()).Return(&agents, nil)

	exporter := util.NewJSONExporter()

	err := run(deps.cmd, &opts{targetArg: "org/1", exporter: exporter})
	require.NoError(t, err)

	var parsed []map[string]any
	err = json.Unmarshal(deps.stdout.Bytes(), &parsed)
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	assert.Equal(t, float64(7), parsed[0]["id"])
	assert.Equal(t, "agent-01", parsed[0]["name"])
	assert.Equal(t, "online", parsed[0]["status"])
}

func TestRunList_JSONOutputEmpty(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	agents := []taskagent.TaskAgent{}
	deps.tac.EXPECT().GetAgents(gomock.Any(), gomock.Any()).Return(&agents, nil)

	exporter := util.NewJSONExporter()

	err := run(deps.cmd, &opts{targetArg: "org/1", exporter: exporter})
	require.NoError(t, err)

	assert.Equal(t, "[]\n", deps.stdout.String())
}

func TestRunList_JSONOutputAllFields(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	createdOn := azuredevops.Time{Time: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)}
	agents := []taskagent.TaskAgent{
		{
			Id:             types.ToPtr(7),
			Name:           types.ToPtr("agent-01"),
			Status:         types.ToPtr(taskagent.TaskAgentStatusValues.Online),
			Enabled:        types.ToPtr(true),
			Version:        types.ToPtr("4.240.0"),
			OsDescription:  types.ToPtr("Linux"),
			MaxParallelism: types.ToPtr(2),
			CreatedOn:      &createdOn,
		},
	}
	deps.tac.EXPECT().GetAgents(gomock.Any(), gomock.Any()).Return(&agents, nil)

	exporter := util.NewJSONExporter()

	err := run(deps.cmd, &opts{targetArg: "org/1", exporter: exporter})
	require.NoError(t, err)

	var parsed []map[string]any
	err = json.Unmarshal(deps.stdout.Bytes(), &parsed)
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	assert.Equal(t, float64(7), parsed[0]["id"])
	assert.Equal(t, "agent-01", parsed[0]["name"])
	assert.Equal(t, "online", parsed[0]["status"])
	assert.Equal(t, true, parsed[0]["enabled"])
	assert.Equal(t, "4.240.0", parsed[0]["version"])
	assert.Equal(t, "Linux", parsed[0]["osDescription"])
	assert.Equal(t, float64(2), parsed[0]["maxParallelism"])
	assert.Contains(t, parsed[0]["createdOn"], "2024-01-15")
}
