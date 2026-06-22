package util_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	util "github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

func newMockCmdContextWithDefaultOrg(t *testing.T, defaultOrg string) util.CmdContext {
	t.Helper()
	return newMockCmdContextForParse(t, defaultOrg, nil, nil)
}

func newMockCmdContextForParse(t *testing.T, defaultOrg string, defaultOrgErr error, configErr error) util.CmdContext {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCtx := mocks.NewMockCmdContext(ctrl)
	if configErr != nil {
		mockCtx.EXPECT().Config().Return(nil, configErr).AnyTimes()
		return mockCtx
	}

	mockConfig := mocks.NewMockConfig(ctrl)
	mockAuth := mocks.NewMockAuthConfig(ctrl)

	mockCtx.EXPECT().Config().Return(mockConfig, nil).AnyTimes()
	mockConfig.EXPECT().Authentication().Return(mockAuth).AnyTimes()
	mockAuth.EXPECT().GetDefaultOrganization().Return(defaultOrg, defaultOrgErr).AnyTimes()

	return mockCtx
}

func TestParseScope(t *testing.T) {
	t.Run("invalid scope format", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockCtx := mocks.NewMockCmdContext(ctrl)

		_, err := util.ParseScope(mockCtx, "org/")
		require.Error(t, err)
	})
}

func TestParseOrganizationArg(t *testing.T) {
	t.Run("project segment not allowed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockCtx := mocks.NewMockCmdContext(ctrl)

		_, err := util.ParseOrganizationArg(mockCtx, "org/project")
		require.Error(t, err)
	})
}

func TestParseProjectScope(t *testing.T) {
	t.Run("invalid project argument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		_, err := util.ParseProjectScope(mocks.NewMockCmdContext(ctrl), "")
		require.Error(t, err)
	})
}

func TestParseTarget(t *testing.T) {
	t.Run("invalid format", func(t *testing.T) {
		_, err := util.ParseTarget("justone")
		require.Error(t, err)
	})
}

func TestParseTargetWithDefaultOrganization(t *testing.T) {
	t.Run("missing default organization", func(t *testing.T) {
		mockCtx := newMockCmdContextWithDefaultOrg(t, "")

		_, err := util.ParseTargetWithDefaultOrganization(mockCtx, "group")
		require.Error(t, err)
	})
}

func TestParseProjectTargetWithDefaultOrganization(t *testing.T) {
	t.Run("missing default organization", func(t *testing.T) {
		mockCtx := newMockCmdContextWithDefaultOrg(t, "")

		_, err := util.ParseProjectTargetWithDefaultOrganization(mockCtx, "project/target")
		require.Error(t, err)
	})

	t.Run("missing project segment", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		_, err := util.ParseProjectTargetWithDefaultOrganization(mocks.NewMockCmdContext(ctrl), "justtarget")
		require.Error(t, err)
	})
}

func TestParse(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		opts          util.ParseOptions
		want          *util.Path
		wantErr       string
		defaultOrg    string
		defaultOrgErr error
		configErr     error
	}{
		{
			name:       "empty input with implicit org",
			raw:        "",
			opts:       util.ParseOptions{AllowImplicitOrg: true},
			want:       &util.Path{Organization: "default-org"},
			defaultOrg: "default-org",
		},
		{
			name:    "empty input without implicit org",
			raw:     "",
			opts:    util.ParseOptions{AllowImplicitOrg: false},
			wantErr: "invalid input \"\": expected at least 1 segments, got 0",
		},
		{
			name: "single segment with implicit org",
			raw:  "myorg",
			opts: util.ParseOptions{AllowImplicitOrg: true},
			want: &util.Path{Organization: "myorg"},
		},
		{
			name: "two segments with implicit org",
			raw:  "myorg/myproject",
			opts: util.ParseOptions{AllowImplicitOrg: true},
			want: &util.Path{Organization: "myorg", Project: "myproject"},
		},
		{
			name: "single segment without implicit org",
			raw:  "myorg",
			opts: util.ParseOptions{AllowImplicitOrg: false},
			want: &util.Path{Organization: "myorg"},
		},
		{
			name: "explicit org with target (no project)",
			raw:  "org/group",
			opts: util.ParseOptions{AllowImplicitOrg: false, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Organization: "org", Targets: []string{"group"}},
		},
		{
			name: "explicit org and project with target",
			raw:  "org/project/group",
			opts: util.ParseOptions{AllowImplicitOrg: false, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Organization: "org", Project: "project", Targets: []string{"group"}},
		},
		{
			name: "project target with implicit org",
			raw:  "project/target",
			opts: util.ParseOptions{AllowImplicitOrg: true, RequireProject: true, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Organization: "default-org", Project: "project", Targets: []string{"target"}},
		},
		{
			name: "target only with implicit org",
			raw:  "target",
			opts: util.ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Organization: "default-org", Targets: []string{"target"}},
		},
		{
			name:    "empty segment",
			raw:     "org/",
			opts:    util.ParseOptions{AllowImplicitOrg: true},
			wantErr: "input \"org/\" contains empty segment",
		},
		{
			name:    "whitespace only input",
			raw:     "  ",
			opts:    util.ParseOptions{AllowImplicitOrg: false},
			wantErr: "invalid input \"  \": expected at least 1 segments, got 0",
		},
		{
			name:    "whitespace segment",
			raw:     "org/ /project",
			opts:    util.ParseOptions{AllowImplicitOrg: true},
			wantErr: "input \"org/ /project\" contains empty segment",
		},
		{
			name: "unbounded targets assign org project and trailing targets",
			raw:  "org/proj/extra",
			opts: util.ParseOptions{AllowImplicitOrg: true},
			want: &util.Path{
				Organization: "org",
				Project:      "proj",
				Targets:      []string{"extra"},
			},
		},
		{
			name: "variable target counts allow one target",
			raw:  "org/project/target",
			opts: util.ParseOptions{AllowImplicitOrg: false, MinTargets: 1, MaxTargets: 2},
			want: &util.Path{
				Organization: "org",
				Project:      "project",
				Targets:      []string{"target"},
			},
		},
		{
			name: "variable target counts allow two targets",
			raw:  "org/project/target/extra",
			opts: util.ParseOptions{AllowImplicitOrg: false, MinTargets: 1, MaxTargets: 2},
			want: &util.Path{
				Organization: "org",
				Project:      "project",
				Targets:      []string{"target", "extra"},
			},
		},
		{
			name: "unbounded targets with required project keep explicit organization",
			raw:  "org/project/target/extra",
			opts: util.ParseOptions{AllowImplicitOrg: true, RequireProject: true, MinTargets: 1},
			want: &util.Path{
				Organization: "org",
				Project:      "project",
				Targets:      []string{"target", "extra"},
			},
		},
		{
			name: "unbounded targets with required project allow implicit organization single target",
			raw:  "project/target",
			opts: util.ParseOptions{AllowImplicitOrg: true, RequireProject: true, MinTargets: 1},
			want: &util.Path{
				Organization: "default-org",
				Project:      "project",
				Targets:      []string{"target"},
			},
			defaultOrg: "default-org",
		},
		{
			name: "unbounded targets with min required and many trailing segments",
			raw:  "org/project/a/b/c",
			opts: util.ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 0},
			want: &util.Path{
				Organization: "org",
				Project:      "project",
				Targets:      []string{"a", "b", "c"},
			},
		},
		{
			name: "bounded targets treat second segment as target when project optional",
			raw:  "org/project",
			opts: util.ParseOptions{AllowImplicitOrg: false, MinTargets: 1, MaxTargets: 2},
			want: &util.Path{
				Organization: "org",
				Targets:      []string{"project"},
			},
		},
		{
			name:    "variable target counts reject too many targets",
			raw:     "org/project/target/extra/extra2",
			opts:    util.ParseOptions{AllowImplicitOrg: false, MinTargets: 1, MaxTargets: 2},
			wantErr: "invalid input \"org/project/target/extra/extra2\": expected 2-4 segments, got 5",
		},
		{
			name: "bounded targets prefer smallest prefix when project optional",
			raw:  "org/a/b/c",
			opts: util.ParseOptions{AllowImplicitOrg: false, MinTargets: 1, MaxTargets: 5},
			want: &util.Path{
				Organization: "org",
				Targets:      []string{"a", "b", "c"},
			},
		},
		{
			name: "bounded targets prefer largest prefix when org optional and project required",
			raw:  "org/project/a/b/c",
			opts: util.ParseOptions{AllowImplicitOrg: true, RequireProject: true, MinTargets: 1, MaxTargets: 5},
			want: &util.Path{
				Organization: "org",
				Project:      "project",
				Targets:      []string{"a", "b", "c"},
			},
		},
		{
			name:    "invalid options max targets less than min targets",
			raw:     "org",
			opts:    util.ParseOptions{MinTargets: 3, MaxTargets: 1},
			wantErr: "invalid options: target range [3,1] is not satisfiable",
		},
		{
			name:    "negative min targets rejected",
			raw:     "org",
			opts:    util.ParseOptions{MinTargets: -1},
			wantErr: "invalid options: target range [-1,0] is not satisfiable",
		},
		{
			name:    "nil ctx when org omitted",
			raw:     "target",
			opts:    util.ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "no organization specified and no default organization configured",
		},
		{
			name:       "default organization lookup returns empty string",
			raw:        "target",
			opts:       util.ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 1},
			wantErr:    "no organization specified and no default organization configured",
			defaultOrg: "",
		},
		{
			name:          "default organization lookup returns error",
			raw:           "target",
			opts:          util.ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 1},
			wantErr:       "no organization specified and no default organization configured: boom",
			defaultOrgErr: errors.New("boom"),
		},
		{
			name:      "config lookup fails",
			raw:       "target",
			opts:      util.ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 1},
			wantErr:   "config boom",
			configErr: errors.New("config boom"),
		},
		{
			name:    "require project with empty input and implicit org",
			raw:     "",
			opts:    util.ParseOptions{AllowImplicitOrg: true, RequireProject: true},
			wantErr: "invalid input \"\": expected at least 1 segments, got 0",
		},
		{
			name: "unbounded with required project implicit org and targets",
			raw:  "project/a/b",
			opts: util.ParseOptions{AllowImplicitOrg: true, RequireProject: true, MinTargets: 0, MaxTargets: 0},
			want: &util.Path{
				Organization: "default-org",
				Project:      "project",
				Targets:      []string{"a", "b"},
			},
			defaultOrg: "default-org",
		},
		{
			name: "unbounded with required project and org and targets",
			raw:  "org/project/a/b",
			opts: util.ParseOptions{AllowImplicitOrg: true, RequireProject: true, MinTargets: 0, MaxTargets: 0},
			want: &util.Path{
				Organization: "org",
				Project:      "project",
				Targets:      []string{"a", "b"},
			},
		},
		{
			name: "unbounded with required project and org and no targets",
			raw:  "org/project",
			opts: util.ParseOptions{AllowImplicitOrg: true, RequireProject: true, MinTargets: 0, MaxTargets: 0},
			want: &util.Path{
				Organization: "org",
				Project:      "project",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx util.CmdContext
			if tt.configErr != nil || tt.defaultOrg != "" || tt.defaultOrgErr != nil {
				ctx = newMockCmdContextForParse(t, tt.defaultOrg, tt.defaultOrgErr, tt.configErr)
			} else if tt.opts.AllowImplicitOrg && tt.wantErr == "" {
				ctx = newMockCmdContextForParse(t, "default-org", nil, nil)
			}

			got, err := util.Parse(ctx, tt.raw, tt.opts)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want.Organization, got.Organization)
			assert.Equal(t, tt.want.Project, got.Project)
			assert.Equal(t, tt.want.Targets, got.Targets)
		})
	}
}

func TestResolveScopeDescriptor_EmptyOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCtx := mocks.NewMockCmdContext(ctrl)

	descriptor, projectID, err := util.ResolveScopeDescriptor(mockCtx, "", "project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organization is required")
	assert.Nil(t, descriptor)
	assert.Nil(t, projectID)
}

func TestResolveScopeDescriptor_NoProject(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCtx := mocks.NewMockCmdContext(ctrl)

	descriptor, projectID, err := util.ResolveScopeDescriptor(mockCtx, "org", "")
	require.NoError(t, err)
	assert.Nil(t, descriptor)
	assert.Nil(t, projectID)
}

func TestResolveScopeDescriptor_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCtx := mocks.NewMockCmdContext(ctrl)
	mockClientFactory := mocks.NewMockClientFactory(ctrl)
	mockCoreClient := mocks.NewMockCoreClient(ctrl)
	mockGraphClient := mocks.NewMockGraphClient(ctrl)

	mockCtx.EXPECT().ClientFactory().Return(mockClientFactory).AnyTimes()
	mockCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	projectID := uuid.New()
	projectRef := &core.TeamProject{
		Id: types.ToPtr(projectID),
	}
	mockClientFactory.EXPECT().
		Core(gomock.Any(), "org").
		Return(mockCoreClient, nil)
	mockCoreClient.EXPECT().
		GetProject(gomock.Any(), gomock.AssignableToTypeOf(core.GetProjectArgs{})).
		Return(projectRef, nil)

	descriptorValue := "vssgp.Descriptor"
	mockClientFactory.EXPECT().
		Graph(gomock.Any(), "org").
		Return(mockGraphClient, nil)
	mockGraphClient.EXPECT().
		GetDescriptor(gomock.Any(), gomock.AssignableToTypeOf(graph.GetDescriptorArgs{})).
		Return(&graph.GraphDescriptorResult{Value: &descriptorValue}, nil)

	descriptor, projectIDPtr, err := util.ResolveScopeDescriptor(mockCtx, "org", "project")
	require.NoError(t, err)
	require.NotNil(t, descriptor)
	assert.Equal(t, descriptorValue, *descriptor)
	require.NotNil(t, projectIDPtr)
	assert.Equal(t, projectID.String(), *projectIDPtr)
}

func TestResolveScopeDescriptor_CoreClientError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCtx := mocks.NewMockCmdContext(ctrl)
	mockClientFactory := mocks.NewMockClientFactory(ctrl)

	mockCtx.EXPECT().ClientFactory().Return(mockClientFactory)
	mockCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	mockClientFactory.EXPECT().
		Core(gomock.Any(), "org").
		Return(nil, errors.New("boom"))

	descriptor, projectID, err := util.ResolveScopeDescriptor(mockCtx, "org", "project")
	require.Error(t, err)
	assert.Nil(t, descriptor)
	assert.Nil(t, projectID)
}
