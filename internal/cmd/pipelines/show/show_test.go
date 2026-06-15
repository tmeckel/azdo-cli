package show

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
)

type dependencies struct {
	cmd         *mocks.MockCmdContext
	clientFact  *mocks.MockClientFactory
	buildClient *mocks.MockBuildClient
	cfg         *mocks.MockConfig
	authCfg     *mocks.MockAuthConfig
	stdout      *bytes.Buffer
	t           *testing.T
}

func newDependencies(t *testing.T, organization string) *dependencies {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	d := &dependencies{
		cmd:         mocks.NewMockCmdContext(ctrl),
		clientFact:  mocks.NewMockClientFactory(ctrl),
		buildClient: mocks.NewMockBuildClient(ctrl),
		cfg:         mocks.NewMockConfig(ctrl),
		authCfg:     mocks.NewMockAuthConfig(ctrl),
		stdout:      out,
		t:           t,
	}

	d.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	d.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	d.cmd.EXPECT().ClientFactory().Return(d.clientFact).AnyTimes()
	d.cmd.EXPECT().Config().Return(d.cfg, nil).AnyTimes()
	d.cfg.EXPECT().Authentication().Return(d.authCfg).AnyTimes()
	d.authCfg.EXPECT().GetDefaultOrganization().Return(organization, nil).AnyTimes()

	return d
}

func (d *dependencies) setupBuildClient() {
	d.clientFact.EXPECT().Build(gomock.Any(), gomock.Any()).Return(d.buildClient, nil).AnyTimes()
}

func (d *dependencies) setupGetDefinition(def *build.BuildDefinition, err error) {
	d.buildClient.EXPECT().GetDefinition(gomock.Any(), gomock.Any()).Return(def, err).Times(1)
}

func (d *dependencies) expectGetDefinition(project string, id int, def *build.BuildDefinition, err error) {
	d.buildClient.EXPECT().GetDefinition(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.GetDefinitionArgs) (*build.BuildDefinition, error) {
			require.NotNil(d.t, args.Project)
			require.Equal(d.t, project, *args.Project)
			require.NotNil(d.t, args.DefinitionId)
			require.Equal(d.t, id, *args.DefinitionId)
			return def, err
		},
	).Times(1)
}

func (d *dependencies) expectGetDefinitionsByName(project, name string, resp *build.GetDefinitionsResponseValue, err error) {
	d.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
			require.NotNil(d.t, args.Project)
			require.Equal(d.t, project, *args.Project)
			require.NotNil(d.t, args.Name)
			require.Equal(d.t, name, *args.Name)
			return resp, err
		},
	).Times(1)
}

func defFromJSON(t *testing.T, j string) *build.BuildDefinition {
	t.Helper()
	var d build.BuildDefinition
	require.NoError(t, json.Unmarshal([]byte(j), &d))
	return &d
}

func sampleDef(t *testing.T) *build.BuildDefinition {
	t.Helper()
	return defFromJSON(t, `{
		"id": 42, "name": "MyPipeline", "revision": 7,
		"description": "<p>My pipeline description</p>",
		"path": "\\", "type": "build",
		"url": "https://dev.azure.com/myorg/fabrikam/_apis/pipelines/definitions/42",
		"_links": {}, "process": {"type": 2, "yamlFilename": "azure-pipelines.yml"},
		"repository": {"id": "repo-id", "name": "MyRepo"},
		"queue": {"id": 1, "name": "Azure Pipelines"},
		"authoredBy": {"displayName": "Alice", "uniqueName": "alice@x.com", "id": "alice-id"},
		"createdDate": "2024-01-01T12:00:00Z", "quality": "definition"
	}`)
}

func TestNewCmd_RegistersAsShowLeaf(t *testing.T) {
	t.Parallel()
	d := newDependencies(t, "MyOrg")
	cmd := NewCmd(d.cmd)
	assert.Equal(t, "show", cmd.Name())
	assert.Contains(t, cmd.Aliases, "view")
	assert.Contains(t, cmd.Aliases, "status")
	assert.Contains(t, cmd.Use, "show [ORGANIZATION/]PROJECT/PIPELINE")
}

func TestNewCmd_RequiresOneArg(t *testing.T) {
	t.Parallel()
	d := newDependencies(t, "MyOrg")
	cmd := NewCmd(d.cmd)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline target is required")
}

func TestRunShow_ResolveByPositiveInteger(t *testing.T) {
	t.Parallel()
	d := newDependencies(t, "MyOrg")
	d.setupBuildClient()
	d.expectGetDefinition("Fabrikam", 42, sampleDef(t), nil)

	opts := &showOptions{scopeArg: "MyOrg/Fabrikam/42"}
	err := runShow(d.cmd, opts)
	require.NoError(t, err)
}

func TestRunShow_RejectsNonPositiveNumericPipelineID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "zero", raw: "0"},
		{name: "negative", raw: "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := newDependencies(t, "MyOrg")
			d.setupBuildClient()

			opts := &showOptions{scopeArg: "MyOrg/Fabrikam/" + tt.raw}
			err := runShow(d.cmd, opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "pipeline id must be greater than zero")
		})
	}
}

func TestRunShow_ResolveByName(t *testing.T) {
	t.Parallel()
	d := newDependencies(t, "MyOrg")
	d.setupBuildClient()

	defID := 42
	defName := "MyPipeline"
	d.expectGetDefinitionsByName("Fabrikam", "MyPipeline", &build.GetDefinitionsResponseValue{
		Value: []build.BuildDefinitionReference{
			{Id: &defID, Name: &defName},
		},
	}, nil)

	d.expectGetDefinition("Fabrikam", 42, sampleDef(t), nil)

	opts := &showOptions{scopeArg: "MyOrg/Fabrikam/MyPipeline"}
	err := runShow(d.cmd, opts)
	require.NoError(t, err)

	output := d.stdout.String()
	assert.Contains(t, output, "MyPipeline")
}

func TestRunShow_NameNotFound(t *testing.T) {
	t.Parallel()
	d := newDependencies(t, "MyOrg")
	d.setupBuildClient()

	d.expectGetDefinitionsByName("Fabrikam", "NonExistent", &build.GetDefinitionsResponseValue{
		Value: []build.BuildDefinitionReference{},
	}, nil)

	opts := &showOptions{scopeArg: "MyOrg/Fabrikam/NonExistent"}
	err := runShow(d.cmd, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRunShow_NameAmbiguous(t *testing.T) {
	t.Parallel()
	d := newDependencies(t, "MyOrg")
	d.setupBuildClient()

	id1, id2 := 1, 2
	n1, n2 := "SameName", "SameName"
	d.expectGetDefinitionsByName("Fabrikam", "SameName", &build.GetDefinitionsResponseValue{
		Value: []build.BuildDefinitionReference{
			{Id: &id1, Name: &n1},
			{Id: &id2, Name: &n2},
		},
	}, nil)

	opts := &showOptions{scopeArg: "MyOrg/Fabrikam/SameName"}
	err := runShow(d.cmd, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
}

func TestRunShow_TemplateOutput_BasicFields(t *testing.T) {
	t.Parallel()
	d := newDependencies(t, "MyOrg")
	d.setupBuildClient()
	d.setupGetDefinition(sampleDef(t), nil)

	opts := &showOptions{scopeArg: "MyOrg/Fabrikam/42"}
	err := runShow(d.cmd, opts)
	require.NoError(t, err)

	output := d.stdout.String()
	assert.Contains(t, output, "id:")
	assert.Contains(t, output, "name:")
	assert.Contains(t, output, "revision:")
	assert.Contains(t, output, "path:")
	assert.Contains(t, output, "type:")
	assert.Contains(t, output, "quality:")
	assert.Contains(t, output, "MyPipeline")
}

func TestRunShow_ResolveByNameQueryError(t *testing.T) {
	t.Parallel()
	d := newDependencies(t, "MyOrg")
	d.setupBuildClient()
	d.expectGetDefinitionsByName("Fabrikam", "MyPipeline", nil, fmt.Errorf("lookup failed"))

	opts := &showOptions{scopeArg: "MyOrg/Fabrikam/MyPipeline"}
	err := runShow(d.cmd, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to query pipeline definitions")
}

func TestRunShow_ResolveByNameWithEmptyDefinitionID(t *testing.T) {
	t.Parallel()
	d := newDependencies(t, "MyOrg")
	d.setupBuildClient()

	defName := "MyPipeline"
	d.expectGetDefinitionsByName("Fabrikam", "MyPipeline", &build.GetDefinitionsResponseValue{
		Value: []build.BuildDefinitionReference{{Name: &defName}},
	}, nil)

	opts := &showOptions{scopeArg: "MyOrg/Fabrikam/MyPipeline"}
	err := runShow(d.cmd, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "returned empty id")
}

func TestRunShow_TemplateOutput_URL(t *testing.T) {
	t.Parallel()
	d := newDependencies(t, "MyOrg")
	d.setupBuildClient()
	d.setupGetDefinition(sampleDef(t), nil)

	opts := &showOptions{scopeArg: "MyOrg/Fabrikam/42"}
	err := runShow(d.cmd, opts)
	require.NoError(t, err)

	output := d.stdout.String()
	assert.Contains(t, output, "url:")
	assert.Contains(t, output, "https://dev.azure.com/myorg/fabrikam/_apis/pipelines/definitions/42")
}

func TestRunShow_TemplateOutput_DescriptionMarkdown(t *testing.T) {
	t.Parallel()
	d := newDependencies(t, "MyOrg")
	d.setupBuildClient()

	defJSON := `{
		"id": 1, "name": "Test", "description": "<p>Hello <strong>World</strong></p>",
		"path": "\\", "type": "build", "_links": {},
		"url": "https://dev.azure.com/myorg/test/_apis/pipelines/definitions/1",
		"quality": "definition"
	}`
	d.setupGetDefinition(defFromJSON(t, defJSON), nil)

	opts := &showOptions{scopeArg: "MyOrg/Fabrikam/1"}
	err := runShow(d.cmd, opts)
	require.NoError(t, err)

	output := d.stdout.String()
	assert.Contains(t, output, "description:")
	assert.Contains(t, output, "Hello")
	assert.Contains(t, output, "World")
}

func TestRunShow_TemplateOutput_NoDescription(t *testing.T) {
	t.Parallel()
	d := newDependencies(t, "MyOrg")
	d.setupBuildClient()

	defJSON := `{
		"id": 1, "name": "Test", "path": "\\", "type": "build",
		"_links": {},
		"url": "https://dev.azure.com/myorg/test/_apis/pipelines/definitions/1",
		"quality": "definition"
	}`
	d.setupGetDefinition(defFromJSON(t, defJSON), nil)

	opts := &showOptions{scopeArg: "MyOrg/Fabrikam/1"}
	err := runShow(d.cmd, opts)
	require.NoError(t, err)

	output := d.stdout.String()
	assert.NotContains(t, output, "description:")
}

func TestRunShow_TemplateOutput_ProcessAndRepository_Nested(t *testing.T) {
	t.Parallel()
	d := newDependencies(t, "MyOrg")
	d.setupBuildClient()
	d.setupGetDefinition(sampleDef(t), nil)

	opts := &showOptions{scopeArg: "MyOrg/Fabrikam/42"}
	err := runShow(d.cmd, opts)
	require.NoError(t, err)

	output := d.stdout.String()
	assert.Contains(t, output, "process:")
	assert.Contains(t, output, "repository:")
	assert.Contains(t, output, "MyRepo")
	assert.Contains(t, output, "Azure Pipelines")
}

func TestRunShow_JSONOutput(t *testing.T) {
	t.Parallel()
	d := newDependencies(t, "MyOrg")
	d.setupBuildClient()
	d.expectGetDefinition("Fabrikam", 42, sampleDef(t), nil)

	cmd := NewCmd(d.cmd)
	cmd.SetArgs([]string{"--json", "MyOrg/Fabrikam/42"})
	err := cmd.Execute()
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(d.stdout.Bytes(), &payload))
	assert.Equal(t, float64(42), payload["id"])
	assert.Equal(t, "MyPipeline", payload["name"])
}

func TestRunShow_ProjectScopeParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		scope   string
		org     string
		wantErr bool
	}{
		{name: "three segments", scope: "MyOrg/Fabrikam/42", org: "MyOrg", wantErr: false},
		{name: "two segments", scope: "Fabrikam/42", org: "Fabrikam", wantErr: false},
		{name: "empty", scope: "", org: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := newDependencies(t, tt.org)

			if !tt.wantErr {
				d.setupBuildClient()
				d.setupGetDefinition(sampleDef(t), nil)
			}

			opts := &showOptions{scopeArg: tt.scope}
			err := runShow(d.cmd, opts)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRunShow_ClientFactoryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	cmd := mocks.NewMockCmdContext(ctrl)
	clientFact := mocks.NewMockClientFactory(ctrl)
	cfg := mocks.NewMockConfig(ctrl)
	authCfg := mocks.NewMockAuthConfig(ctrl)

	io, _, _, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmd.EXPECT().ClientFactory().Return(clientFact).AnyTimes()
	cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(authCfg).AnyTimes()
	authCfg.EXPECT().GetDefaultOrganization().Return("MyOrg", nil).AnyTimes()

	clientFact.EXPECT().Build(gomock.Any(), gomock.Any()).Return(nil, assert.AnError).Times(1)

	opts := &showOptions{scopeArg: "MyOrg/Fabrikam/42"}
	err := runShow(cmd, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create Build client")
}

func TestRunShow_APIFetchError(t *testing.T) {
	t.Parallel()
	d := newDependencies(t, "MyOrg")
	d.setupBuildClient()
	d.setupGetDefinition(nil, fmt.Errorf("API returned status 500"))

	opts := &showOptions{scopeArg: "MyOrg/Fabrikam/42"}
	err := runShow(d.cmd, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch pipeline definition")
}

func TestRunShow_OrganizationFromConfigDefault(t *testing.T) {
	t.Parallel()
	d := newDependencies(t, "DefaultOrg")
	d.setupBuildClient()
	d.setupGetDefinition(sampleDef(t), nil)

	opts := &showOptions{scopeArg: "Fabrikam/42"}
	err := runShow(d.cmd, opts)
	require.NoError(t, err)

	output := d.stdout.String()
	assert.Contains(t, output, "MyPipeline")
}
