package list

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
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
	ctrl        *gomock.Controller
	cmd         *mocks.MockCmdContext
	clientFact  *mocks.MockClientFactory
	buildClient *mocks.MockBuildClient
	stdout      *bytes.Buffer
	stderr      *bytes.Buffer
}

func setupFakeDeps(t *testing.T, organization string) *fakeListDeps {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, errOut := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &fakeListDeps{
		ctrl:        ctrl,
		cmd:         mocks.NewMockCmdContext(ctrl),
		clientFact:  mocks.NewMockClientFactory(ctrl),
		buildClient: mocks.NewMockBuildClient(ctrl),
		stdout:      out,
		stderr:      errOut,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.clientFact.EXPECT().Build(gomock.Any(), organization).Return(deps.buildClient, nil).AnyTimes()

	return deps
}

func sampleDefinition(id int, name, path string, quality build.DefinitionQuality, queueStatus build.DefinitionQueueStatus) build.BuildDefinitionReference {
	def := build.BuildDefinitionReference{
		Id:          types.ToPtr(id),
		Name:        types.ToPtr(name),
		Path:        types.ToPtr(path),
		Type:        types.ToPtr(build.DefinitionTypeValues.Build),
		Quality:     types.ToPtr(quality),
		QueueStatus: types.ToPtr(queueStatus),
		Revision:    types.ToPtr(1),
	}
	if queueStatus == build.DefinitionQueueStatusValues.Enabled {
		def.Queue = &build.AgentPoolQueue{Name: types.ToPtr("default")}
	}
	return def
}

func TestNewCmd_RegistersAsListLeaf(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "list [ORGANIZATION/]PROJECT", cmd.Use)
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

	require.NotNil(t, f.Lookup("name"))
	require.NotNil(t, f.Lookup("repository"))
	require.NotNil(t, f.Lookup("repository-type"))
	require.NotNil(t, f.Lookup("top"))
	require.NotNil(t, f.Lookup("folder-path"))
	require.NotNil(t, f.Lookup("query-order"))
	require.NotNil(t, f.Lookup("max-items"))
	assert.NotNil(t, f.Lookup("json"))
	assert.NotNil(t, f.Lookup("jq"))
	assert.NotNil(t, f.Lookup("template"))
}

func TestRunList_BasicCall(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "myorg")
	defs := []build.BuildDefinitionReference{
		sampleDefinition(7, "pipeline-a", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
		sampleDefinition(8, "pipeline-b", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Paused),
	}
	deps.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).Return(
		&build.GetDefinitionsResponseValue{Value: defs}, nil,
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = runList(deps.cmd, &opts{scope: "myorg/myproject"})
	require.NoError(t, err)

	output := deps.stdout.String()
	assert.Contains(t, output, "7")
	assert.Contains(t, output, "pipeline-a")
	assert.Contains(t, output, "8")
	assert.Contains(t, output, "pipeline-b")
	assert.Contains(t, output, "enabled")
	assert.Contains(t, output, "paused")
	assert.Contains(t, output, "default")
}

func TestRunList_OrgFromArg(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "explicit-org")
	defs := []build.BuildDefinitionReference{
		sampleDefinition(1, "pipe-1", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
	}
	deps.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).Return(
		&build.GetDefinitionsResponseValue{Value: defs}, nil,
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = runList(deps.cmd, &opts{scope: "explicit-org/myproject"})
	require.NoError(t, err)
	assert.Contains(t, deps.stdout.String(), "pipe-1")
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

	err := runList(cmd, &opts{scope: "myproject"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no organization specified")
}

func TestRunList_InvalidFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    opts
		wantMsg string
	}{
		{"negative top", opts{scope: "org/proj", top: -1}, "invalid --top"},
		{"negative max-items", opts{scope: "org/proj", maxItems: -5}, "invalid --max-items"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := setupFakeDeps(t, "org")
			err := runList(deps.cmd, &tt.opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
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

	clientFact.EXPECT().Build(gomock.Any(), "myorg").Return(nil, fmt.Errorf("connection failed"))

	err := runList(cmd, &opts{scope: "myproject"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection failed")
}

func TestRunList_SDKError(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	deps.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("API error"))

	err := runList(deps.cmd, &opts{scope: "org/proj"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
}

func TestRunList_EmptyResult(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	deps.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).Return(
		&build.GetDefinitionsResponseValue{Value: []build.BuildDefinitionReference{}}, nil,
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = runList(deps.cmd, &opts{scope: "org/proj"})
	require.NoError(t, err)
}

func TestRunList_FilterByName(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	defs := []build.BuildDefinitionReference{
		sampleDefinition(1, "my-pipeline", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
	}
	deps.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
			require.NotNil(t, args.Name)
			assert.Equal(t, "my-pipeline", *args.Name)
			return &build.GetDefinitionsResponseValue{Value: defs}, nil
		},
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = runList(deps.cmd, &opts{scope: "org/proj", name: "my-pipeline"})
	require.NoError(t, err)
}

func TestRunList_FolderPathFilter(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	defs := []build.BuildDefinitionReference{
		sampleDefinition(1, "pipe", "\\user1\\prod", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
	}
	deps.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
			require.NotNil(t, args.Path)
			assert.Equal(t, "user1/production", *args.Path)
			return &build.GetDefinitionsResponseValue{Value: defs}, nil
		},
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = runList(deps.cmd, &opts{scope: "org/proj", folderPath: "user1/production"})
	require.NoError(t, err)
}

func TestRunList_RepositoryFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     opts
		wantType string
	}{
		{"default type tfsgit", opts{scope: "org/proj", repository: "my-repo"}, "tfsgit"},
		{"explicit type github", opts{scope: "org/proj", repository: "my-repo", repositoryType: "github"}, "github"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := setupFakeDeps(t, "org")
			defs := []build.BuildDefinitionReference{
				sampleDefinition(1, "pipe", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
			}
			deps.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, args build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
					require.NotNil(t, args.RepositoryId)
					assert.Equal(t, "my-repo", *args.RepositoryId)
					require.NotNil(t, args.RepositoryType)
					assert.Equal(t, tt.wantType, *args.RepositoryType)
					return &build.GetDefinitionsResponseValue{Value: defs}, nil
				},
			)
			tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
			require.NoError(t, err)
			deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()
			err = runList(deps.cmd, &tt.opts)
			require.NoError(t, err)
		})
	}
}

func TestRunList_MaxItemsFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		maxItems int
		defs     []build.BuildDefinitionReference
		want     []string
		notWant  []string
	}{
		{
			"caps at limit",
			1,
			[]build.BuildDefinitionReference{
				sampleDefinition(1, "p1", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
				sampleDefinition(2, "p2", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
				sampleDefinition(3, "p3", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
			},
			[]string{"p1"},
			[]string{"p2", "p3"},
		},
		{
			"exceeds result count",
			100,
			[]build.BuildDefinitionReference{
				sampleDefinition(1, "p1", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
				sampleDefinition(2, "p2", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
			},
			[]string{"p1", "p2"},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := setupFakeDeps(t, "org")
			deps.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).Return(
				&build.GetDefinitionsResponseValue{Value: tt.defs}, nil,
			)
			tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
			require.NoError(t, err)
			deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()
			err = runList(deps.cmd, &opts{scope: "org/proj", maxItems: tt.maxItems})
			require.NoError(t, err)
			output := deps.stdout.String()
			for _, w := range tt.want {
				assert.Contains(t, output, w)
			}
			for _, n := range tt.notWant {
				assert.NotContains(t, output, n)
			}
		})
	}
}

func TestRunList_DraftColumnPresent(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	defs := []build.BuildDefinitionReference{
		sampleDefinition(1, "draft-pipe", "\\", build.DefinitionQualityValues.Draft, build.DefinitionQueueStatusValues.Enabled),
		sampleDefinition(2, "normal-pipe", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
	}
	deps.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).Return(
		&build.GetDefinitionsResponseValue{Value: defs}, nil,
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = runList(deps.cmd, &opts{scope: "org/proj"})
	require.NoError(t, err)

	assert.Contains(t, deps.stdout.String(), "*")
}

func TestRunList_DraftColumnAbsent(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	defs := []build.BuildDefinitionReference{
		sampleDefinition(1, "pipe-1", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
	}
	deps.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).Return(
		&build.GetDefinitionsResponseValue{Value: defs}, nil,
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = runList(deps.cmd, &opts{scope: "org/proj"})
	require.NoError(t, err)

	assert.NotContains(t, deps.stdout.String(), "*")
}

func TestRunList_JSONOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		defs  []build.BuildDefinitionReference
		check func(t *testing.T, out []byte)
	}{
		{
			"empty",
			[]build.BuildDefinitionReference{},
			func(t *testing.T, out []byte) {
				assert.Equal(t, "[]\n", string(out))
			},
		},
		{
			"minimal fields from sample",
			[]build.BuildDefinitionReference{
				sampleDefinition(7, "pipeline-a", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
			},
			func(t *testing.T, out []byte) {
				var parsed []map[string]any
				err := json.Unmarshal(out, &parsed)
				require.NoError(t, err)
				require.Len(t, parsed, 1)
				assert.Equal(t, float64(7), parsed[0]["id"])
				assert.Equal(t, "pipeline-a", parsed[0]["name"])
				assert.Equal(t, "build", parsed[0]["type"])
				assert.Equal(t, "definition", parsed[0]["quality"])
				assert.Equal(t, "enabled", parsed[0]["queueStatus"])
				assert.Equal(t, float64(1), parsed[0]["revision"])
				queue, ok := parsed[0]["queue"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "default", queue["name"])
			},
		},
		{
			"full raw struct with all fields",
			[]build.BuildDefinitionReference{
				{
					Id:          types.ToPtr(42),
					Name:        types.ToPtr("full-pipeline"),
					Path:        types.ToPtr("\\team\\services"),
					Type:        types.ToPtr(build.DefinitionTypeValues.Build),
					Quality:     types.ToPtr(build.DefinitionQualityValues.Draft),
					QueueStatus: types.ToPtr(build.DefinitionQueueStatusValues.Paused),
					Revision:    types.ToPtr(3),
					Queue:       &build.AgentPoolQueue{Name: types.ToPtr("linux-pool")},
				},
			},
			func(t *testing.T, out []byte) {
				var parsed []map[string]any
				err := json.Unmarshal(out, &parsed)
				require.NoError(t, err)
				require.Len(t, parsed, 1)
				assert.Equal(t, float64(42), parsed[0]["id"])
				assert.Equal(t, "full-pipeline", parsed[0]["name"])
				assert.Equal(t, "\\team\\services", parsed[0]["path"])
				assert.Equal(t, float64(3), parsed[0]["revision"])
				assert.Equal(t, "build", parsed[0]["type"])
				assert.Equal(t, "draft", parsed[0]["quality"])
				assert.Equal(t, "paused", parsed[0]["queueStatus"])
				queue, ok := parsed[0]["queue"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "linux-pool", queue["name"])
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := setupFakeDeps(t, "org")
			deps.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).Return(
				&build.GetDefinitionsResponseValue{Value: tt.defs}, nil,
			)
			exporter := util.NewJSONExporter()
			err := runList(deps.cmd, &opts{scope: "org/proj", exporter: exporter})
			require.NoError(t, err)
			tt.check(t, deps.stdout.Bytes())
		})
	}
}

func TestRunList_QueryOrderFlag(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	defs := []build.BuildDefinitionReference{
		sampleDefinition(1, "pipe", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
	}
	deps.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
			require.NotNil(t, args.QueryOrder)
			assert.Equal(t, build.DefinitionQueryOrderValues.DefinitionNameAscending, *args.QueryOrder)
			return &build.GetDefinitionsResponseValue{Value: defs}, nil
		},
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = runList(deps.cmd, &opts{scope: "org/proj", queryOrder: "definitionNameAscending"})
	require.NoError(t, err)
}

func TestRunList_SortsByIDAscending(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	unsorted := []build.BuildDefinitionReference{
		sampleDefinition(3, "c", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
		sampleDefinition(1, "a", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
		sampleDefinition(2, "b", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
	}
	deps.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).Return(
		&build.GetDefinitionsResponseValue{Value: unsorted}, nil,
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = runList(deps.cmd, &opts{scope: "org/proj"})
	require.NoError(t, err)

	output := deps.stdout.String()
	aIdx := strings.Index(output, "a")
	bIdx := strings.Index(output, "b")
	cIdx := strings.Index(output, "c")
	assert.True(t, aIdx < bIdx && bIdx < cIdx, "definitions should appear sorted by ID")
}

func TestRunList_Pagination(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	page1 := []build.BuildDefinitionReference{
		sampleDefinition(1, "p1", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
	}
	page2 := []build.BuildDefinitionReference{
		sampleDefinition(2, "p2", "\\", build.DefinitionQualityValues.Definition, build.DefinitionQueueStatusValues.Enabled),
	}
	gomock.InOrder(
		deps.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).Return(
			&build.GetDefinitionsResponseValue{Value: page1, ContinuationToken: "token-1"}, nil,
		),
		deps.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, args build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
				require.NotNil(t, args.ContinuationToken)
				assert.Equal(t, "token-1", *args.ContinuationToken)
				return &build.GetDefinitionsResponseValue{Value: page2}, nil
			},
		),
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	err = runList(deps.cmd, &opts{scope: "org/proj"})
	require.NoError(t, err)

	output := deps.stdout.String()
	assert.Contains(t, output, "p1")
	assert.Contains(t, output, "p2")
}
