package list

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
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

type dependencies struct {
	ctrl       *gomock.Controller
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	taskClient *mocks.MockTaskAgentClient
	config     *mocks.MockConfig
	auth       *mocks.MockAuthConfig
	stdout     *bytes.Buffer
}

func newDependencies(t *testing.T, organization string) *dependencies {
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

func (d *dependencies) setupDefaultOrg(org string) {
	d.config = mocks.NewMockConfig(d.ctrl)
	d.auth = mocks.NewMockAuthConfig(d.ctrl)
	d.cmd.EXPECT().Config().Return(d.config, nil).AnyTimes()
	d.config.EXPECT().Authentication().Return(d.auth).AnyTimes()
	d.auth.EXPECT().GetDefaultOrganization().Return(org, nil).AnyTimes()
}

func sampleQueue(id int, name, poolName, projectID string) taskagent.TaskAgentQueue {
	queue := taskagent.TaskAgentQueue{
		Id:   types.ToPtr(id),
		Name: types.ToPtr(name),
		Pool: &taskagent.TaskAgentPoolReference{
			Id:   types.ToPtr(id + 100),
			Name: types.ToPtr(poolName),
		},
	}
	if projectID != "" {
		queue.ProjectId = types.ToPtr(uuid.MustParse(projectID))
	}
	return queue
}

func TestNewCmd(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "list [ORGANIZATION/]PROJECT", cmd.Use)
	assert.ElementsMatch(t, []string{"ls", "l"}, cmd.Aliases)
	assert.NotNil(t, cmd.RunE)
	assert.Equal(t, "", cmd.Flag("action-filter").DefValue)

	err := cmd.ParseFlags([]string{"--action-filter", "view"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "valid values are {")
	assert.Contains(t, err.Error(), "none")
	assert.Contains(t, err.Error(), "manage")
	assert.Contains(t, err.Error(), "use")
}

func TestRun_ScopeResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		scope        string
		defaultOrg   string
		taskAgentOrg string
	}{
		{name: "explicit org", scope: "myorg/Fabrikam", taskAgentOrg: "myorg"},
		{name: "default org", scope: "Fabrikam", defaultOrg: "default-org", taskAgentOrg: "default-org"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := newDependencies(t, "")
			if tt.defaultOrg != "" {
				deps.setupDefaultOrg(tt.defaultOrg)
			}
			deps.clientFact.EXPECT().TaskAgent(gomock.Any(), tt.taskAgentOrg).Return(deps.taskClient, nil).AnyTimes()

			queues := []taskagent.TaskAgentQueue{sampleQueue(1, "Default", "pool-1", "")}
			deps.taskClient.EXPECT().GetAgentQueues(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, args taskagent.GetAgentQueuesArgs) (*[]taskagent.TaskAgentQueue, error) {
					require.NotNil(t, args.Project)
					assert.Equal(t, "Fabrikam", *args.Project)
					return &queues, nil
				},
			)

			tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
			require.NoError(t, err)
			deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

			err = run(deps.cmd, &opts{scope: tt.scope})
			require.NoError(t, err)
		})
	}
}

func TestRun_InvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    opts
		wantErr string
	}{
		{name: "invalid project scope", opts: opts{scope: "org/project/extra"}, wantErr: "invalid project argument"},
		{name: "negative max items", opts: opts{scope: "org/project", maxItems: -1}, wantErr: "invalid --max-items"},
		{name: "invalid action filter", opts: opts{scope: "org/project", actionFilter: types.ToPtr("view")}, wantErr: "invalid action filter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := newDependencies(t, "org")
			err := run(deps.cmd, &tt.opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestRun_ClientFactoryError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "")
	deps.setupDefaultOrg("org")
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "org").Return(nil, fmt.Errorf("connection failed"))

	err := run(deps.cmd, &opts{scope: "project"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection failed")
}

func TestRun_SDKError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "org")
	deps.taskClient.EXPECT().GetAgentQueues(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("API error"))

	err := run(deps.cmd, &opts{scope: "org/project"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
}

func TestRun_ActionFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		actionFilter *string
		want         *taskagent.TaskAgentQueueActionFilter
	}{
		{name: "manage", actionFilter: types.ToPtr("manage"), want: types.ToPtr(taskagent.TaskAgentQueueActionFilterValues.Manage)},
		{name: "omitted", actionFilter: nil, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := newDependencies(t, "org")
			queues := []taskagent.TaskAgentQueue{sampleQueue(1, "Default", "pool-1", "")}
			deps.taskClient.EXPECT().GetAgentQueues(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, args taskagent.GetAgentQueuesArgs) (*[]taskagent.TaskAgentQueue, error) {
					assert.Equal(t, tt.want, args.ActionFilter)
					return &queues, nil
				},
			)

			tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
			require.NoError(t, err)
			deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

			err = run(deps.cmd, &opts{scope: "org/project", actionFilter: tt.actionFilter})
			require.NoError(t, err)
		})
	}
}

func TestRun_BasicCall(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	queues := []taskagent.TaskAgentQueue{sampleQueue(7, "Default", "pool-1", "11111111-1111-1111-1111-111111111111")}
	deps.taskClient.EXPECT().GetAgentQueues(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args taskagent.GetAgentQueuesArgs) (*[]taskagent.TaskAgentQueue, error) {
			require.NotNil(t, args.Project)
			assert.Equal(t, "Fabrikam", *args.Project)
			assert.Nil(t, args.QueueName)
			return &queues, nil
		},
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = run(deps.cmd, &opts{scope: "myorg/Fabrikam"})
	require.NoError(t, err)

	assert.Contains(t, deps.stdout.String(), "7\tDefault\tpool-1\t11111111-1111-1111-1111-111111111111")
}

func TestRun_FilterByNameAndMaxItemsCap(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "org")
	queues := []taskagent.TaskAgentQueue{
		sampleQueue(1, "Default", "pool-1", ""),
		sampleQueue(2, "Default Copy", "pool-2", ""),
	}
	deps.taskClient.EXPECT().GetAgentQueues(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args taskagent.GetAgentQueuesArgs) (*[]taskagent.TaskAgentQueue, error) {
			require.NotNil(t, args.QueueName)
			assert.Equal(t, "Default", *args.QueueName)
			return &queues, nil
		},
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = run(deps.cmd, &opts{scope: "org/project", name: "Default", maxItems: 1})
	require.NoError(t, err)

	assert.Contains(t, deps.stdout.String(), "Default")
	assert.NotContains(t, deps.stdout.String(), "Default Copy")
}

func TestRun_JSONOutput(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "org")
	queues := []taskagent.TaskAgentQueue{sampleQueue(7, "Default", "pool-1", "11111111-1111-1111-1111-111111111111")}
	deps.taskClient.EXPECT().GetAgentQueues(gomock.Any(), gomock.Any()).Return(&queues, nil)

	exporter := util.NewJSONExporter()
	err := run(deps.cmd, &opts{scope: "org/project", exporter: exporter})
	require.NoError(t, err)

	var parsed []map[string]any
	err = json.Unmarshal(deps.stdout.Bytes(), &parsed)
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	assert.Equal(t, float64(7), parsed[0]["id"])
	assert.Equal(t, "Default", parsed[0]["name"])

	pool := parsed[0]["pool"].(map[string]any)
	assert.Equal(t, float64(107), pool["id"])
	assert.Equal(t, "pool-1", pool["name"])
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", parsed[0]["projectId"])
	assert.NotContains(t, parsed[0], "poolName")
}
