package shared

import (
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
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

var ansiEscapeRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func newJSONExporterForArgs(t *testing.T, args []string) util.Exporter {
	t.Helper()

	cmd := &cobra.Command{Run: func(*cobra.Command, []string) {}}
	cmd.Flags().Bool("web", false, "")

	var exporter util.Exporter
	util.AddJSONFlags(cmd, &exporter, ServiceEndpointJSONFields)

	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	_, err := cmd.ExecuteC()
	require.NoError(t, err)
	require.NotNil(t, exporter)
	return exporter
}

type recordingExporter struct {
	wrote bool
	data  any
}

func (e *recordingExporter) Fields() []string { return nil }

func (e *recordingExporter) Write(ios *iostreams.IOStreams, data any) error {
	e.wrote = true
	e.data = data
	_, err := ios.Out.Write([]byte("exported\n"))
	return err
}

func TestOutput_ReturnsErrorWhenIOStreamsFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	cmdCtx.EXPECT().IOStreams().Return(nil, errors.New("boom")).Times(1)

	err := Output(cmdCtx, &serviceendpoint.ServiceEndpoint{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestOutput_UsesExporterWhenProvided(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	ios, _, out, _ := iostreams.Test()
	cmdCtx.EXPECT().IOStreams().Return(ios, nil).Times(1)

	epID := uuid.New()
	endpoint := &serviceendpoint.ServiceEndpoint{
		Id:   &epID,
		Name: types.ToPtr("example"),
	}

	exp := &recordingExporter{}
	err := Output(cmdCtx, endpoint, exp)
	require.NoError(t, err)

	assert.True(t, exp.wrote)
	assert.Same(t, endpoint, exp.data)

	got := out.String()
	assert.Contains(t, got, "exported")
	assert.NotContains(t, got, "ID:")
}

func TestOutput_JSONExporter_RespectsJSONFieldSelection(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		expectedKeys []string
	}{
		{
			name:         "default selection includes all allowed fields",
			args:         []string{"--json"},
			expectedKeys: ServiceEndpointJSONFields,
		},
		{
			name:         "explicit include selects only specified fields",
			args:         []string{"--json=id,name"},
			expectedKeys: []string{"id", "name"},
		},
		{
			name:         "dash prefix excludes fields from default selection",
			args:         []string{"--json=-authorization,-data"},
			expectedKeys: excludeServiceEndpointFields(ServiceEndpointJSONFields, "authorization", "data"),
		},
		{
			name:         "explicit include then exclude removes field",
			args:         []string{"--json=id,name,-name"},
			expectedKeys: []string{"id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cmdCtx := mocks.NewMockCmdContext(ctrl)
			ios, _, out, _ := iostreams.Test()
			cmdCtx.EXPECT().IOStreams().Return(ios, nil).Times(1)

			endpointID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
			groupScopeID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
			projectID := uuid.MustParse("00000000-0000-0000-0000-000000000003")

			endpointData := map[string]string{"env": "prod"}
			authParams := map[string]string{"tenant": "abc"}
			projectRefs := []serviceendpoint.ServiceEndpointProjectReference{
				{
					Name:        types.ToPtr("proj-ref"),
					Description: types.ToPtr("shared"),
					ProjectReference: &serviceendpoint.ProjectReference{
						Id:   &projectID,
						Name: types.ToPtr("MyProject"),
					},
				},
			}

			endpoint := &serviceendpoint.ServiceEndpoint{
				AdministratorsGroup: &webapi.IdentityRef{
					DisplayName: types.ToPtr("Admins"),
					UniqueName:  types.ToPtr("admins@example.com"),
				},
				Authorization: &serviceendpoint.EndpointAuthorization{
					Scheme:     types.ToPtr("OAuth"),
					Parameters: &authParams,
				},
				CreatedBy: &webapi.IdentityRef{
					DisplayName: types.ToPtr("Alice"),
					UniqueName:  types.ToPtr("alice@example.com"),
				},
				Data:         &endpointData,
				Description:  types.ToPtr("desc"),
				GroupScopeId: &groupScopeID,
				Id:           &endpointID,
				IsReady:      types.ToPtr(true),
				IsShared:     types.ToPtr(false),
				Name:         types.ToPtr("example"),
				OperationStatus: map[string]string{
					"state": "ok",
				},
				Owner:                            types.ToPtr("library"),
				ReadersGroup:                     &webapi.IdentityRef{DisplayName: types.ToPtr("Readers"), UniqueName: types.ToPtr("readers@example.com")},
				ServiceEndpointProjectReferences: &projectRefs,
				Type:                             types.ToPtr("generic"),
				Url:                              types.ToPtr("http://localhost"),
			}

			exp := newJSONExporterForArgs(t, tt.args)
			err := Output(cmdCtx, endpoint, exp)
			require.NoError(t, err)

			var decoded map[string]any
			require.NoError(t, json.Unmarshal(out.Bytes(), &decoded))

			require.Len(t, decoded, len(tt.expectedKeys))
			for _, k := range tt.expectedKeys {
				assert.Contains(t, decoded, k)
			}
		})
	}
}

func TestOutput_RendersTemplateWhenNoExporter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	ios, _, out, _ := iostreams.Test()
	cmdCtx.EXPECT().IOStreams().Return(ios, nil).Times(1)

	epID := uuid.New()
	data := map[string]string{"env": "prod"}
	params := map[string]string{"tenant": "abc"}

	endpoint := &serviceendpoint.ServiceEndpoint{
		Id:          &epID,
		Name:        types.ToPtr("my endpoint"),
		Type:        types.ToPtr("azurerm"),
		Description: types.ToPtr("desc"),
		Owner:       types.ToPtr("   "), // should be suppressed by hasText
		IsReady:     types.ToPtr(true),
		IsShared:    nil, // b helper should render empty string
		Url:         nil, // should be suppressed by hasText
		CreatedBy: &webapi.IdentityRef{
			DisplayName: types.ToPtr("Alice"),
			UniqueName:  types.ToPtr("alice@example.com"),
		},
		Data: &data,
		Authorization: &serviceendpoint.EndpointAuthorization{
			Scheme:     types.ToPtr("OAuth"),
			Parameters: &params,
		},
	}

	err := Output(cmdCtx, endpoint, nil)
	require.NoError(t, err)

	got := ansiEscapeRE.ReplaceAllString(out.String(), "")

	assert.Contains(t, got, "ID: "+epID.String())
	assert.Contains(t, got, "Name: my endpoint")
	assert.Contains(t, got, "Type: azurerm")
	assert.Contains(t, got, "Description: desc")
	assert.Contains(t, got, "IsReady: true")
	assert.Contains(t, got, "IsShared:")

	assert.NotContains(t, got, "Owner:")
	assert.NotContains(t, got, "URL:")

	assert.Contains(t, got, "Created By: Alice (alice@example.com)")
	assert.Contains(t, got, "Data:")
	assert.Contains(t, got, "env:")

	assert.Contains(t, got, "Authorization:")
	assert.Contains(t, got, "Scheme:")
	assert.Contains(t, got, "tenant:")

	// sanity check: template should not emit empty lines for whitespace-only Owner
	assert.False(t, strings.Contains(got, "Owner:"))
}

func excludeServiceEndpointFields(fields []string, excluded ...string) []string {
	result := slices.Clone(fields)
	result = slices.DeleteFunc(result, func(v string) bool {
		for _, x := range excluded {
			if v == x {
				return true
			}
		}
		return false
	})
	return result
}
