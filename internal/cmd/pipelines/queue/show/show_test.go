package show

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type dependencies struct {
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

func newDependencies(t *testing.T) *dependencies {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(true)
	io.SetStderrTTY(true)

	deps := &dependencies{
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

func (d *dependencies) setupDefaultOrg(org string) {
	d.cmd.EXPECT().Config().Return(d.config, nil).AnyTimes()
	d.config.EXPECT().Authentication().Return(d.auth).AnyTimes()
	d.auth.EXPECT().GetDefaultOrganization().Return(org, nil).AnyTimes()
}

func sampleQueue() *taskagent.TaskAgentQueue {
	return &taskagent.TaskAgentQueue{
		Id:        types.ToPtr(7),
		Name:      types.ToPtr("Default"),
		ProjectId: types.ToPtr(uuid.MustParse("11111111-1111-1111-1111-111111111111")),
		Pool: &taskagent.TaskAgentPoolReference{
			Id:   types.ToPtr(42),
			Name: types.ToPtr("pool-1"),
		},
	}
}

func TestNewCmd_RegistersAsShowLeaf(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "show", cmd.Name())
	assert.Contains(t, cmd.Aliases, "view")
	assert.Contains(t, cmd.Aliases, "status")
	assert.True(t, strings.HasPrefix(cmd.Use, "show [ORGANIZATION/]PROJECT/QUEUE"))
	assert.NotNil(t, cmd.RunE)
}

func TestNewCmd_RequiresOneArg(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.setupDefaultOrg("myorg")

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "queue target is required")
}

func TestRunShow_ResolveByPositiveInteger(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	deps.taskClient.EXPECT().GetAgentQueue(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args taskagent.GetAgentQueueArgs) (*taskagent.TaskAgentQueue, error) {
			assert.Equal(t, 7, *args.QueueId)
			return sampleQueue(), nil
		})

	opts := &showOptions{targetArg: "myorg/Fabrikam/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)
}

func TestRunShow_ResolveByName(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	queueList := []taskagent.TaskAgentQueue{
		{Id: types.ToPtr(7), Name: types.ToPtr("Default")},
	}
	deps.taskClient.EXPECT().GetAgentQueues(gomock.Any(), gomock.Any()).
		Return(&queueList, nil)
	deps.taskClient.EXPECT().GetAgentQueue(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args taskagent.GetAgentQueueArgs) (*taskagent.TaskAgentQueue, error) {
			assert.Equal(t, 7, *args.QueueId)
			return sampleQueue(), nil
		})

	opts := &showOptions{targetArg: "myorg/Fabrikam/Default"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)
}

func TestRunShow_TemplateOutput_BasicFields(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)
	deps.taskClient.EXPECT().GetAgentQueue(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args taskagent.GetAgentQueueArgs) (*taskagent.TaskAgentQueue, error) {
			assert.Equal(t, 7, *args.QueueId)
			assert.Equal(t, "Fabrikam", *args.Project)
			return sampleQueue(), nil
		})

	opts := &showOptions{targetArg: "myorg/Fabrikam/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, "id: 7")
	assert.Contains(t, output, "name: Default")
	assert.Contains(t, output, "project id: 11111111-1111-1111-1111-111111111111")
}

func TestRunShow_InvalidQueueID(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	opts := &showOptions{targetArg: "myorg/Fabrikam/0"}
	err := runShow(deps.cmd, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid queue id 0")
}

func TestRunShow_TemplateOutput_Pool_Nested(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	q := sampleQueue()
	q.Pool = &taskagent.TaskAgentPoolReference{
		Id:   types.ToPtr(42),
		Name: types.ToPtr("pool-1"),
	}
	deps.taskClient.EXPECT().GetAgentQueue(gomock.Any(), gomock.Any()).Return(q, nil)

	opts := &showOptions{targetArg: "myorg/Fabrikam/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, "pool: 42 (pool-1)")
}

func TestRunShow_TemplateOutput_NoPool(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)

	q := sampleQueue()
	q.Pool = nil
	deps.taskClient.EXPECT().GetAgentQueue(gomock.Any(), gomock.Any()).Return(q, nil)

	opts := &showOptions{targetArg: "myorg/Fabrikam/7"}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.NotContains(t, output, "pool:")
}

func TestRunShow_JSONOutput(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)
	deps.taskClient.EXPECT().GetAgentQueue(gomock.Any(), gomock.Any()).Return(sampleQueue(), nil)

	exporter := util.NewJSONExporter()
	opts := &showOptions{targetArg: "myorg/Fabrikam/7", exporter: exporter}
	err := runShow(deps.cmd, opts)
	require.NoError(t, err)

	output := cleanOutput(deps.stdout)
	assert.Contains(t, output, `"id":7`)
	assert.Contains(t, output, `"name":"Default"`)
	assert.Contains(t, output, `"projectId":"11111111-1111-1111-1111-111111111111"`)
	assert.NotContains(t, output, `"url":`)
	assert.NotContains(t, output, `"createdBy":`)
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
			targetArg: "myorg/Fabrikam/7",
			wantOrg:   "myorg",
		},
		{
			name:       "implicit org from config",
			targetArg:  "Fabrikam/7",
			defaultOrg: "default-org",
			wantOrg:    "default-org",
		},
		{
			name:      "invalid input with too many segments",
			targetArg: "org/proj/extra/7",
			wantErr:   "invalid input",
		},
		{
			name:      "missing project segment",
			targetArg: "7",
			wantErr:   "invalid input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := newDependencies(t)
			if tt.defaultOrg != "" {
				deps.setupDefaultOrg(tt.defaultOrg)
			}

			if tt.wantErr == "" {
				deps.clientFact.EXPECT().TaskAgent(gomock.Any(), tt.wantOrg).Return(deps.taskClient, nil)
				deps.taskClient.EXPECT().GetAgentQueue(gomock.Any(), gomock.Any()).Return(sampleQueue(), nil)
			}

			opts := &showOptions{targetArg: tt.targetArg}
			err := runShow(deps.cmd, opts)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRunShow_ClientFactoryError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	expectedErr := fmt.Errorf("connection failed")
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(nil, expectedErr)

	opts := &showOptions{targetArg: "myorg/Fabrikam/7"}
	err := runShow(deps.cmd, opts)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestRunShow_ResolveByName_ListError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)
	expectedErr := fmt.Errorf("list failed")
	deps.taskClient.EXPECT().GetAgentQueues(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

	opts := &showOptions{targetArg: "myorg/Fabrikam/Default"}
	err := runShow(deps.cmd, opts)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestRunShow_ResolveByName_MultipleMatches(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)
	queueList := []taskagent.TaskAgentQueue{
		{Id: types.ToPtr(7), Name: types.ToPtr("Default")},
		{Id: types.ToPtr(8), Name: types.ToPtr("default")},
	}
	deps.taskClient.EXPECT().GetAgentQueues(gomock.Any(), gomock.Any()).Return(&queueList, nil)

	opts := &showOptions{targetArg: "myorg/Fabrikam/Default"}
	err := runShow(deps.cmd, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple queues named")
}

func TestRunShow_ResolveByName_MissingID(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)
	queueList := []taskagent.TaskAgentQueue{
		{Name: types.ToPtr("Default")},
	}
	deps.taskClient.EXPECT().GetAgentQueues(gomock.Any(), gomock.Any()).Return(&queueList, nil)

	opts := &showOptions{targetArg: "myorg/Fabrikam/Default"}
	err := runShow(deps.cmd, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `queue "Default" returned without an ID`)
}

func TestRunShow_ResolveByName_NotFound(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)
	queueList := []taskagent.TaskAgentQueue{{Id: types.ToPtr(7), Name: types.ToPtr("Other")}}
	deps.taskClient.EXPECT().GetAgentQueues(gomock.Any(), gomock.Any()).Return(&queueList, nil)

	opts := &showOptions{targetArg: "myorg/Fabrikam/Default"}
	err := runShow(deps.cmd, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `queue "Default" not found`)
}

func TestRunShow_SDKError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t)
	deps.clientFact.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(deps.taskClient, nil)
	expectedErr := fmt.Errorf("API error")
	deps.taskClient.EXPECT().GetAgentQueue(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

	opts := &showOptions{targetArg: "myorg/Fabrikam/7"}
	err := runShow(deps.cmd, opts)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestNewCmd_HasFlags(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	jsonFlag := cmd.Flag("json")
	require.NotNil(t, jsonFlag)
}

func TestNewCmd_DoesNotExposeRawFlag(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Nil(t, cmd.Flag("raw"))
}
