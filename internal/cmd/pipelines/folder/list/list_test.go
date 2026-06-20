package list

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
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
	buildCli   *mocks.MockBuildClient
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
		buildCli:   mocks.NewMockBuildClient(ctrl),
		stdout:     out,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	if organization != "" {
		deps.clientFact.EXPECT().Build(gomock.Any(), organization).Return(deps.buildCli, nil).AnyTimes()
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

func sampleFolder(path, description string) build.Folder {
	return build.Folder{
		Path:        types.ToPtr(path),
		Description: types.ToPtr(description),
	}
}

func sampleFolderWithMetadata(path, description string) build.Folder {
	createdOn := azuredevops.Time{}
	lastChangedDate := azuredevops.Time{}

	return build.Folder{
		CreatedOn:       &createdOn,
		Description:     types.ToPtr(description),
		LastChangedDate: &lastChangedDate,
		Path:            types.ToPtr(path),
		Project: &core.TeamProjectReference{
			Name: types.ToPtr("Fabrikam"),
		},
	}
}

func TestNewCmd_list(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "list [ORGANIZATION/]PROJECT", cmd.Use)
	assert.ElementsMatch(t, []string{"ls", "l"}, cmd.Aliases)
	assert.NotNil(t, cmd.RunE)

	f := cmd.Flags()
	assert.NotNil(t, f.Lookup("path"))
	assert.NotNil(t, f.Lookup("query-order"))
	assert.NotNil(t, f.Lookup("max-items"))
	assert.NotNil(t, f.Lookup("json"))
	assert.NotNil(t, f.Lookup("jq"))
	assert.NotNil(t, f.Lookup("template"))
}

func TestNewCmd_invalidQueryOrder(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	cmd.SetArgs([]string{"Fabrikam", "--query-order", "banana"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "valid values are {asc|desc}")
}

func TestNewCmd_missingProject(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestRunList_success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		opts         opts
		returned     []build.Folder
		assertArgs   func(*testing.T, build.GetFoldersArgs)
		assertOutput func(*testing.T, string)
	}{
		{
			name: "no path",
			opts: opts{targetArg: "myorg/Fabrikam"},
			returned: []build.Folder{
				sampleFolder("P/Foo", "Foo folder"),
				sampleFolder("P/Bar", "Bar folder"),
			},
			assertArgs: func(t *testing.T, args build.GetFoldersArgs) {
				t.Helper()
				require.NotNil(t, args.Project)
				assert.Equal(t, "Fabrikam", *args.Project)
				assert.Nil(t, args.Path)
				assert.Nil(t, args.QueryOrder)
			},
			assertOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "P/Foo")
				assert.Contains(t, output, "P/Bar")
				assert.Contains(t, output, "Foo folder")
				assert.Contains(t, output, "Bar folder")
			},
		},
		{
			name:     "with path",
			opts:     opts{targetArg: "myorg/Fabrikam", path: "P"},
			returned: []build.Folder{sampleFolder("P/Foo", "")},
			assertArgs: func(t *testing.T, args build.GetFoldersArgs) {
				t.Helper()
				require.NotNil(t, args.Path)
				assert.Equal(t, "P", *args.Path)
			},
			assertOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "P/Foo")
			},
		},
		{
			name:     "query order asc",
			opts:     opts{targetArg: "myorg/Fabrikam", queryOrder: "asc"},
			returned: []build.Folder{sampleFolder("P/Foo", "")},
			assertArgs: func(t *testing.T, args build.GetFoldersArgs) {
				t.Helper()
				require.NotNil(t, args.QueryOrder)
				assert.Equal(t, build.FolderQueryOrderValues.FolderAscending, *args.QueryOrder)
			},
			assertOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "P/Foo")
			},
		},
		{
			name:     "query order desc",
			opts:     opts{targetArg: "myorg/Fabrikam", queryOrder: "desc"},
			returned: []build.Folder{sampleFolder("P/Foo", "")},
			assertArgs: func(t *testing.T, args build.GetFoldersArgs) {
				t.Helper()
				require.NotNil(t, args.QueryOrder)
				assert.Equal(t, build.FolderQueryOrderValues.FolderDescending, *args.QueryOrder)
			},
			assertOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "P/Foo")
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, "myorg")
			deps.buildCli.EXPECT().GetFolders(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, args build.GetFoldersArgs) (*[]build.Folder, error) {
					tc.assertArgs(t, args)
					return &tc.returned, nil
				},
			)

			tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
			require.NoError(t, err)
			deps.cmd.EXPECT().Printer("list").Return(tp, nil)

			err = runList(deps.cmd, &tc.opts)
			require.NoError(t, err)

			tc.assertOutput(t, deps.stdout.String())
		})
	}
}

func TestRunList_success_maxItems(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	folders := []build.Folder{
		sampleFolder("P/First", ""),
		sampleFolder("P/Second", ""),
		sampleFolder("P/Third", ""),
		sampleFolder("P/Fourth", ""),
		sampleFolder("P/Fifth", ""),
	}
	deps.buildCli.EXPECT().GetFolders(gomock.Any(), gomock.Any()).Return(&folders, nil)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = runList(deps.cmd, &opts{targetArg: "myorg/Fabrikam", maxItems: 2})
	require.NoError(t, err)

	output := deps.stdout.String()
	assert.Contains(t, output, "P/First")
	assert.Contains(t, output, "P/Second")
	assert.NotContains(t, output, "P/Third")
	assert.NotContains(t, output, "P/Fourth")
	assert.NotContains(t, output, "P/Fifth")
}

func TestRunList_success_JSON(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	folders := []build.Folder{
		sampleFolderWithMetadata("P/Foo", "Foo folder"),
		sampleFolder("P/Bar", "Bar folder"),
	}
	deps.buildCli.EXPECT().GetFolders(gomock.Any(), gomock.Any()).Return(&folders, nil)
	deps.cmd.EXPECT().Printer(gomock.Any()).Times(0)

	exporter := util.NewJSONExporter()
	err := runList(deps.cmd, &opts{targetArg: "myorg/Fabrikam", exporter: exporter})
	require.NoError(t, err)

	var parsed []build.Folder
	err = json.Unmarshal(deps.stdout.Bytes(), &parsed)
	require.NoError(t, err)
	require.Len(t, parsed, 2)

	require.NotNil(t, parsed[0].Path)
	require.NotNil(t, parsed[0].Description)
	require.NotNil(t, parsed[0].CreatedOn)
	require.NotNil(t, parsed[0].LastChangedDate)
	require.NotNil(t, parsed[0].Project)
	require.NotNil(t, parsed[0].Project.Name)
	require.NotNil(t, parsed[1].Path)
	require.NotNil(t, parsed[1].Description)

	assert.Equal(t, "P/Foo", *parsed[0].Path)
	assert.Equal(t, "Foo folder", *parsed[0].Description)
	assert.Equal(t, "Fabrikam", *parsed[0].Project.Name)
	assert.Equal(t, "P/Bar", *parsed[1].Path)
	assert.Equal(t, "Bar folder", *parsed[1].Description)
}

func TestRunList_empty(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.buildCli.EXPECT().GetFolders(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ build.GetFoldersArgs) (*[]build.Folder, error) {
			empty := []build.Folder{}
			return &empty, nil
		},
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = runList(deps.cmd, &opts{targetArg: "myorg/Fabrikam"})
	require.NoError(t, err)
	assert.Equal(t, "\n", deps.stdout.String())
}

func TestRunList_APIError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.buildCli.EXPECT().GetFolders(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("boom"))

	err := runList(deps.cmd, &opts{targetArg: "myorg/Fabrikam"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestRunList_missingProject(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "")
	deps.setupDefaultOrg("myorg")

	err := runList(deps.cmd, &opts{targetArg: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid project argument")
}

func TestRunList_invalidMaxItems(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")

	err := runList(deps.cmd, &opts{targetArg: "myorg/Fabrikam", maxItems: -1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--max-items must be >= 0")
}

func TestRunList_clientFactoryError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "")
	deps.setupDefaultOrg("myorg")
	deps.clientFact.EXPECT().Build(gomock.Any(), "myorg").Return(nil, fmt.Errorf("connection failed"))

	err := runList(deps.cmd, &opts{targetArg: "Fabrikam"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection failed")
}

func TestRunList_defaultOrg(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "")
	deps.setupDefaultOrg("default-org")
	deps.clientFact.EXPECT().Build(gomock.Any(), "default-org").Return(deps.buildCli, nil).AnyTimes()

	folders := []build.Folder{sampleFolder("P/Foo", "Foo")}
	deps.buildCli.EXPECT().GetFolders(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.GetFoldersArgs) (*[]build.Folder, error) {
			assert.Equal(t, "Fabrikam", *args.Project)
			return &folders, nil
		},
	)

	tp, err := printer.NewTablePrinter(deps.stdout, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = runList(deps.cmd, &opts{targetArg: "Fabrikam"})
	require.NoError(t, err)

	assert.Contains(t, deps.stdout.String(), "P/Foo")
}
