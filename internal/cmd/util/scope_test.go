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

// defaultOrgCtx returns a CmdContext whose configuration resolves the default
// organization to "default-org".
func defaultOrgCtx(t *testing.T) util.CmdContext {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCtx := mocks.NewMockCmdContext(ctrl)
	mockConfig := mocks.NewMockConfig(ctrl)
	mockAuth := mocks.NewMockAuthConfig(ctrl)

	mockCtx.EXPECT().Config().Return(mockConfig, nil).AnyTimes()
	mockConfig.EXPECT().Authentication().Return(mockAuth).AnyTimes()
	mockAuth.EXPECT().GetDefaultOrganization().Return("default-org", nil).AnyTimes()

	return mockCtx
}

// emptyOrgCtx returns a CmdContext whose configuration resolves the default
// organization to an empty string.
func emptyOrgCtx(t *testing.T) util.CmdContext {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCtx := mocks.NewMockCmdContext(ctrl)
	mockConfig := mocks.NewMockConfig(ctrl)
	mockAuth := mocks.NewMockAuthConfig(ctrl)

	mockCtx.EXPECT().Config().Return(mockConfig, nil).AnyTimes()
	mockConfig.EXPECT().Authentication().Return(mockAuth).AnyTimes()
	mockAuth.EXPECT().GetDefaultOrganization().Return("", nil).AnyTimes()

	return mockCtx
}

// parseCase drives a single table-driven parse expectation.
type parseCase struct {
	name    string
	raw     string
	ctx     util.CmdContext
	opts    util.ParseOptions
	want    *util.Path
	wantErr string
}

func runParseCase(t *testing.T, tt parseCase, parse func(util.CmdContext, string) (*util.Path, error)) {
	t.Helper()
	t.Run(tt.name, func(t *testing.T) {
		got, err := parse(tt.ctx, tt.raw)
		if tt.wantErr != "" {
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			return
		}
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, tt.want, got)
	})
}

func TestParseScope(t *testing.T) {
	ctx := defaultOrgCtx(t)
	tests := []parseCase{
		{
			name: "empty input resolves default organization",
			raw:  "",
			ctx:  ctx,
			want: &util.Path{Organization: "default-org"},
		},
		{
			name: "project uses default organization",
			raw:  "myproject",
			ctx:  ctx,
			want: &util.Path{Organization: "default-org", Project: "myproject"},
		},
		{
			name: "explicit organization with project",
			raw:  "myorg:myproject",
			ctx:  ctx,
			want: &util.Path{Organization: "myorg", Project: "myproject"},
		},
		{
			name: "explicit organization only",
			raw:  "myorg:",
			ctx:  ctx,
			want: &util.Path{Organization: "myorg"},
		},
		{
			name: "bare segment is a project not an organization",
			raw:  "myorg",
			ctx:  ctx,
			want: &util.Path{Organization: "default-org", Project: "myorg"},
		},
		{
			name:    "legacy organization slash project is rejected",
			raw:     "org/project",
			ctx:     ctx,
			wantErr: "legacy ORGANIZATION/... form is not supported, use ORG: syntax",
		},
		{
			name:    "legacy three segment form is rejected",
			raw:     "org/project/extra",
			ctx:     ctx,
			wantErr: "legacy ORGANIZATION/... form is not supported, use ORG: syntax",
		},
		{
			name:    "targets are not allowed",
			raw:     "myorg:myproject/extra",
			ctx:     ctx,
			wantErr: "targets are not allowed",
		},
		{
			name:    "no-project marker is not valid without targets",
			raw:     "/myproject",
			ctx:     ctx,
			wantErr: "targets are not allowed",
		},
		{
			name:    "misplaced colon is rejected",
			raw:     "org/proj:ect",
			ctx:     ctx,
			wantErr: "colon must directly follow the organization",
		},
		{
			name:    "multiple colons are rejected",
			raw:     "org::proj",
			ctx:     ctx,
			wantErr: "contains multiple colons",
		},
		{
			name:    "empty organization prefix is rejected",
			raw:     ":project",
			ctx:     ctx,
			wantErr: "organization must not be empty",
		},
		{
			name:    "trailing slash is rejected",
			raw:     "myproject/",
			ctx:     ctx,
			wantErr: "contains empty segment",
		},
		{
			name: "whitespace around segments is trimmed",
			raw:  " myorg : myproject ",
			ctx:  ctx,
			want: &util.Path{Organization: "myorg", Project: "myproject"},
		},
		{
			name:    "empty input without default organization errors",
			raw:     "",
			ctx:     nil,
			wantErr: "no organization specified and no default organization configured",
		},
		{
			name:    "empty configured default organization errors",
			raw:     "project",
			ctx:     emptyOrgCtx(t),
			wantErr: "no organization specified and no default organization configured",
		},
	}

	for _, tt := range tests {
		runParseCase(t, tt, util.ParseScope)
	}
}

func TestParseOrganizationArg(t *testing.T) {
	ctx := defaultOrgCtx(t)
	tests := []parseCase{
		{
			name: "bare organization",
			raw:  "myorg",
			ctx:  ctx,
			want: &util.Path{Organization: "myorg"},
		},
		{
			name: "organization with trailing colon",
			raw:  "myorg:",
			ctx:  ctx,
			want: &util.Path{Organization: "myorg"},
		},
		{
			name: "empty input resolves default organization",
			raw:  "",
			ctx:  ctx,
			want: &util.Path{Organization: "default-org"},
		},
		{
			name: "whitespace around organization is trimmed",
			raw:  "  myorg  ",
			ctx:  ctx,
			want: &util.Path{Organization: "myorg"},
		},
		{
			name:    "project segment is not allowed",
			raw:     "org/project",
			ctx:     ctx,
			wantErr: "project scope not allowed for this command",
		},
		{
			name:    "colon project form is not allowed",
			raw:     "org:project",
			ctx:     ctx,
			wantErr: "project scope not allowed for this command",
		},
		{
			name:    "no-project marker is not allowed",
			raw:     "org:/target",
			ctx:     ctx,
			wantErr: "expected ORG or ORG: syntax",
		},
		{
			name:    "leading slash is not allowed",
			raw:     "/org",
			ctx:     ctx,
			wantErr: "expected ORG or ORG: syntax",
		},
		{
			name:    "multiple colons are rejected",
			raw:     "org::x",
			ctx:     ctx,
			wantErr: "contains multiple colons",
		},
		{
			name:    "empty organization prefix is rejected",
			raw:     ":org",
			ctx:     ctx,
			wantErr: "organization must not be empty",
		},
		{
			name:    "empty input without default organization errors",
			raw:     "",
			ctx:     nil,
			wantErr: "no organization specified and no default organization configured",
		},
		{
			name:    "empty configured default organization errors",
			raw:     "",
			ctx:     emptyOrgCtx(t),
			wantErr: "no organization specified and no default organization configured",
		},
	}

	for _, tt := range tests {
		runParseCase(t, tt, func(ctx util.CmdContext, raw string) (*util.Path, error) {
			org, err := util.ParseOrganizationArg(ctx, raw)
			if err != nil {
				return nil, err
			}
			return &util.Path{Organization: org}, nil
		})
	}
}

func TestParseProjectScope(t *testing.T) {
	ctx := defaultOrgCtx(t)
	tests := []parseCase{
		{
			name: "project uses default organization",
			raw:  "myproject",
			ctx:  ctx,
			want: &util.Path{Organization: "default-org", Project: "myproject"},
		},
		{
			name: "explicit organization and project",
			raw:  "myorg:myproject",
			ctx:  ctx,
			want: &util.Path{Organization: "myorg", Project: "myproject"},
		},
		{
			name:    "empty input misses project",
			raw:     "",
			ctx:     ctx,
			wantErr: "project is required",
		},
		{
			name:    "organization only misses project",
			raw:     "myorg:",
			ctx:     ctx,
			wantErr: "project is required",
		},
		{
			name:    "legacy organization slash project is rejected",
			raw:     "org/project",
			ctx:     ctx,
			wantErr: "legacy ORGANIZATION/... form is not supported, use ORG: syntax",
		},
		{
			name:    "extra segment is rejected",
			raw:     "org:project/extra",
			ctx:     ctx,
			wantErr: "targets are not allowed",
		},
		{
			name:    "no-project marker misses project",
			raw:     "/project",
			ctx:     ctx,
			wantErr: "project is required",
		},
		{
			name:    "project without default organization errors",
			raw:     "myproject",
			ctx:     nil,
			wantErr: "no organization specified and no default organization configured",
		},
	}

	for _, tt := range tests {
		runParseCase(t, tt, util.ParseProjectScope)
	}
}

func TestParseTarget(t *testing.T) {
	tests := []parseCase{
		{
			name: "organization and target without project",
			raw:  "myorg:/target",
			want: &util.Path{Organization: "myorg", Targets: []string{"target"}},
		},
		{
			name: "organization project and target",
			raw:  "myorg:myproject/target",
			want: &util.Path{Organization: "myorg", Project: "myproject", Targets: []string{"target"}},
		},
		{
			name:    "legacy organization slash target is rejected",
			raw:     "org/target",
			wantErr: "explicit organization is required, use ORG: syntax",
		},
		{
			name:    "legacy organization project target is rejected",
			raw:     "org/project/target",
			wantErr: "explicit organization is required, use ORG: syntax",
		},
		{
			name:    "no-project marker without organization is rejected",
			raw:     "/target",
			wantErr: "explicit organization is required, use ORG: syntax",
		},
		{
			name:    "bare target is rejected",
			raw:     "target",
			wantErr: "explicit organization is required, use ORG: syntax",
		},
		{
			name:    "organization only requires the no-project marker",
			raw:     "myorg:",
			wantErr: "use ORG:/ to specify an organization without a project or targets",
		},
		{
			name:    "zero targets are rejected",
			raw:     "myorg:/",
			wantErr: "expected exactly 1 targets, got 0",
		},
		{
			name:    "too many targets are rejected",
			raw:     "myorg:/target/extra",
			wantErr: "expected exactly 1 targets, got 2",
		},
		{
			name:    "too many targets with project are rejected",
			raw:     "myorg:project/target/extra",
			wantErr: "expected exactly 1 targets, got 2",
		},
		{
			name:    "multiple colons are rejected",
			raw:     "org::target",
			wantErr: "contains multiple colons",
		},
	}

	for _, tt := range tests {
		runParseCase(t, tt, func(ctx util.CmdContext, raw string) (*util.Path, error) {
			return util.ParseTarget(raw)
		})
	}
}

func TestParseTargetWithDefaultOrganization(t *testing.T) {
	ctx := defaultOrgCtx(t)
	tests := []parseCase{
		{
			name: "no-project marker uses default organization",
			raw:  "/target",
			ctx:  ctx,
			want: &util.Path{Organization: "default-org", Targets: []string{"target"}},
		},
		{
			name: "project and target use default organization",
			raw:  "project/target",
			ctx:  ctx,
			want: &util.Path{Organization: "default-org", Project: "project", Targets: []string{"target"}},
		},
		{
			name: "explicit organization with target",
			raw:  "myorg:/target",
			ctx:  ctx,
			want: &util.Path{Organization: "myorg", Targets: []string{"target"}},
		},
		{
			name: "explicit organization with project and target",
			raw:  "myorg:project/target",
			ctx:  ctx,
			want: &util.Path{Organization: "myorg", Project: "project", Targets: []string{"target"}},
		},
		{
			name: "legacy organization slash subject is project first",
			raw:  "org/group",
			ctx:  ctx,
			want: &util.Path{Organization: "default-org", Project: "org", Targets: []string{"group"}},
		},
		{
			name:    "single segment is a project not a target",
			raw:     "target",
			ctx:     ctx,
			wantErr: "expected exactly 1 targets, got 0",
		},
		{
			name:    "legacy organization project target is rejected",
			raw:     "org/project/target",
			ctx:     ctx,
			wantErr: "legacy ORGANIZATION/... form is not supported, use ORG: syntax",
		},
		{
			name:    "empty input misses target",
			raw:     "",
			ctx:     ctx,
			wantErr: "expected exactly 1 targets, got 0",
		},
		{
			name:    "organization only requires the no-project marker",
			raw:     "org:",
			ctx:     ctx,
			wantErr: "use ORG:/ to specify an organization without a project or targets",
		},
		{
			name:    "too many targets are rejected",
			raw:     "/a/b",
			ctx:     ctx,
			wantErr: "expected exactly 1 targets, got 2",
		},
		{
			name:    "no-project target without default organization errors",
			raw:     "/target",
			ctx:     nil,
			wantErr: "no organization specified and no default organization configured",
		},
		{
			name:    "project target without default organization errors",
			raw:     "project/target",
			ctx:     nil,
			wantErr: "no organization specified and no default organization configured",
		},
	}

	for _, tt := range tests {
		runParseCase(t, tt, util.ParseTargetWithDefaultOrganization)
	}
}

func TestParseProjectTargetWithDefaultOrganization(t *testing.T) {
	ctx := defaultOrgCtx(t)
	tests := []parseCase{
		{
			name: "project and target use default organization",
			raw:  "project/target",
			ctx:  ctx,
			want: &util.Path{Organization: "default-org", Project: "project", Targets: []string{"target"}},
		},
		{
			name: "explicit organization with project and target",
			raw:  "myorg:project/target",
			ctx:  ctx,
			want: &util.Path{Organization: "myorg", Project: "project", Targets: []string{"target"}},
		},
		{
			name:    "legacy organization project target is rejected",
			raw:     "org/project/target",
			ctx:     ctx,
			wantErr: "legacy ORGANIZATION/... form is not supported, use ORG: syntax",
		},
		{
			name:    "no-project marker misses project",
			raw:     "/target",
			ctx:     ctx,
			wantErr: "project is required",
		},
		{
			name:    "single segment misses target",
			raw:     "target",
			ctx:     ctx,
			wantErr: "expected exactly 1 targets, got 0",
		},
		{
			name:    "organization only misses project",
			raw:     "org:",
			ctx:     ctx,
			wantErr: "project is required",
		},
		{
			name:    "empty input misses project",
			raw:     "",
			ctx:     ctx,
			wantErr: "project is required",
		},
		{
			name:    "too many targets are rejected",
			raw:     "org:project/target/extra",
			ctx:     ctx,
			wantErr: "expected exactly 1 targets, got 2",
		},
		{
			name:    "project target without default organization errors",
			raw:     "project/target",
			ctx:     nil,
			wantErr: "no organization specified and no default organization configured",
		},
	}

	for _, tt := range tests {
		runParseCase(t, tt, util.ParseProjectTargetWithDefaultOrganization)
	}
}

func TestParseProjectPathTargetWithDefaultOrganization(t *testing.T) {
	ctx := defaultOrgCtx(t)
	tests := []parseCase{
		{
			name: "project and path use default organization",
			raw:  "project/path",
			ctx:  ctx,
			want: &util.Path{Organization: "default-org", Project: "project", Targets: []string{"path"}},
		},
		{
			name: "explicit organization with project and path",
			raw:  "myorg:project/path",
			ctx:  ctx,
			want: &util.Path{Organization: "myorg", Project: "project", Targets: []string{"path"}},
		},
		{
			name: "nested path keeps every segment",
			raw:  "myorg:project/path/to/folder",
			ctx:  ctx,
			want: &util.Path{Organization: "myorg", Project: "project", Targets: []string{"path", "to", "folder"}},
		},
		{
			name:    "single segment misses path",
			raw:     "project",
			ctx:     ctx,
			wantErr: "expected at least 1 targets, got 0",
		},
		{
			name:    "no-project marker misses project",
			raw:     "/path",
			ctx:     ctx,
			wantErr: "project is required",
		},
		{
			name:    "organization only misses project",
			raw:     "myorg:",
			ctx:     ctx,
			wantErr: "project is required",
		},
		{
			name:    "empty input misses project",
			raw:     "",
			ctx:     ctx,
			wantErr: "project is required",
		},
		{
			name:    "project path without default organization errors",
			raw:     "project/path",
			ctx:     nil,
			wantErr: "no organization specified and no default organization configured",
		},
	}

	for _, tt := range tests {
		runParseCase(t, tt, util.ParseProjectPathTargetWithDefaultOrganization)
	}
}

func TestParsePoolAgentTargetWithDefaultOrganization(t *testing.T) {
	ctx := defaultOrgCtx(t)
	tests := []parseCase{
		{
			name: "pool agent uses default organization",
			raw:  "/pool/agent",
			ctx:  ctx,
			want: &util.Path{Organization: "default-org", Targets: []string{"pool", "agent"}},
		},
		{
			name: "explicit organization with pool and agent",
			raw:  "myorg:/pool/agent",
			ctx:  ctx,
			want: &util.Path{Organization: "myorg", Targets: []string{"pool", "agent"}},
		},
		{
			name:    "legacy pool agent slash form is rejected",
			raw:     "pool/agent",
			ctx:     ctx,
			wantErr: "project is not allowed, use the / no-project marker",
		},
		{
			name:    "legacy organization pool agent form is rejected",
			raw:     "org/pool/agent",
			ctx:     ctx,
			wantErr: "project is not allowed, use the / no-project marker",
		},
		{
			name:    "bare segment is rejected without marker",
			raw:     "pool",
			ctx:     ctx,
			wantErr: "project is not allowed, use the / no-project marker",
		},
		{
			name:    "single target is rejected",
			raw:     "/pool",
			ctx:     ctx,
			wantErr: "expected exactly 2 targets, got 1",
		},
		{
			name:    "three targets are rejected",
			raw:     "/pool/agent/extra",
			ctx:     ctx,
			wantErr: "expected exactly 2 targets, got 3",
		},
		{
			name:    "organization only requires the no-project marker",
			raw:     "org:",
			ctx:     ctx,
			wantErr: "use ORG:/ to specify an organization without a project or targets",
		},
		{
			name:    "empty input misses targets",
			raw:     "",
			ctx:     ctx,
			wantErr: "expected exactly 2 targets, got 0",
		},
		{
			name:    "pool agent without default organization errors",
			raw:     "/pool/agent",
			ctx:     nil,
			wantErr: "no organization specified and no default organization configured",
		},
	}

	for _, tt := range tests {
		runParseCase(t, tt, util.ParsePoolAgentTargetWithDefaultOrganization)
	}
}

func TestParse(t *testing.T) {
	ctx := defaultOrgCtx(t)
	tests := []parseCase{
		{
			name: "empty input with implicit organization",
			raw:  "",
			opts: util.ParseOptions{AllowImplicitOrg: true},
			want: &util.Path{Organization: "default-org"},
		},
		{
			name:    "empty input without implicit organization",
			raw:     "",
			opts:    util.ParseOptions{AllowImplicitOrg: false},
			wantErr: "explicit organization is required, use ORG: syntax",
		},
		{
			name: "bare segment is a project with implicit organization",
			raw:  "myorg",
			opts: util.ParseOptions{AllowImplicitOrg: true},
			want: &util.Path{Organization: "default-org", Project: "myorg"},
		},
		{
			name: "explicit organization and project",
			raw:  "myorg:myproject",
			opts: util.ParseOptions{AllowImplicitOrg: true},
			want: &util.Path{Organization: "myorg", Project: "myproject"},
		},
		{
			name: "explicit organization and project without implicit organization",
			raw:  "myorg:myproject",
			opts: util.ParseOptions{AllowImplicitOrg: false},
			want: &util.Path{Organization: "myorg", Project: "myproject"},
		},
		{
			name: "explicit organization only with targets disallowed",
			raw:  "myorg:",
			opts: util.ParseOptions{AllowImplicitOrg: true, DisallowTargets: true},
			want: &util.Path{Organization: "myorg"},
		},
		{
			name: "explicit organization with target and no project",
			raw:  "org:/group",
			opts: util.ParseOptions{AllowImplicitOrg: false, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Organization: "org", Targets: []string{"group"}},
		},
		{
			name: "explicit organization with project and target",
			raw:  "org:project/group",
			opts: util.ParseOptions{AllowImplicitOrg: false, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Organization: "org", Project: "project", Targets: []string{"group"}},
		},
		{
			name: "no-project target uses default organization",
			raw:  "/group",
			opts: util.ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Organization: "default-org", Targets: []string{"group"}},
		},
		{
			name: "project target uses default organization",
			raw:  "project/group",
			opts: util.ParseOptions{AllowImplicitOrg: true, RequireProject: true, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Organization: "default-org", Project: "project", Targets: []string{"group"}},
		},
		{
			name: "project target with optional project",
			raw:  "project/group",
			opts: util.ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Organization: "default-org", Project: "project", Targets: []string{"group"}},
		},
		{
			name: "legacy organization slash subject is project first",
			raw:  "org/group",
			opts: util.ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Organization: "default-org", Project: "org", Targets: []string{"group"}},
		},
		{
			name:    "legacy organization project target is rejected",
			raw:     "org/project/group",
			opts:    util.ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "legacy ORGANIZATION/... form is not supported, use ORG: syntax",
		},
		{
			name:    "explicit organization required without prefix",
			raw:     "project/group",
			opts:    util.ParseOptions{AllowImplicitOrg: false, MinTargets: 1, MaxTargets: 1},
			wantErr: "explicit organization is required, use ORG: syntax",
		},
		{
			name:    "organization only requires marker when targets allowed",
			raw:     "org:",
			opts:    util.ParseOptions{AllowImplicitOrg: true, MinTargets: 0, MaxTargets: 0},
			wantErr: "use ORG:/ to specify an organization without a project or targets",
		},
		{
			name: "no-project marker with zero targets allowed",
			raw:  "/",
			opts: util.ParseOptions{AllowImplicitOrg: true, MinTargets: 0, MaxTargets: 0},
			want: &util.Path{Organization: "default-org"},
		},
		{
			name: "explicit organization marker with zero targets allowed",
			raw:  "org:/",
			opts: util.ParseOptions{AllowImplicitOrg: true, MinTargets: 0, MaxTargets: 0},
			want: &util.Path{Organization: "org"},
		},
		{
			name: "zero targets with project allowed",
			raw:  "project",
			opts: util.ParseOptions{AllowImplicitOrg: true, MinTargets: 0, MaxTargets: 2},
			want: &util.Path{Organization: "default-org", Project: "project"},
		},
		{
			name: "unbounded targets with required project",
			raw:  "project/target/extra",
			opts: util.ParseOptions{AllowImplicitOrg: true, RequireProject: true, MinTargets: 1, MaxTargets: 0},
			want: &util.Path{Organization: "default-org", Project: "project", Targets: []string{"target", "extra"}},
		},
		{
			name: "unbounded targets with explicit organization",
			raw:  "org:project/target/extra",
			opts: util.ParseOptions{AllowImplicitOrg: true, RequireProject: true, MinTargets: 1, MaxTargets: 0},
			want: &util.Path{Organization: "org", Project: "project", Targets: []string{"target", "extra"}},
		},
		{
			name: "unbounded targets with disallowed project",
			raw:  "/a/b/c",
			opts: util.ParseOptions{AllowImplicitOrg: true, DisallowProject: true, MinTargets: 1, MaxTargets: 0},
			want: &util.Path{Organization: "default-org", Targets: []string{"a", "b", "c"}},
		},
		{
			name: "legacy organization project target is project first when unbounded",
			raw:  "org/project/target",
			opts: util.ParseOptions{AllowImplicitOrg: true, RequireProject: true, MinTargets: 1, MaxTargets: 0},
			want: &util.Path{Organization: "default-org", Project: "org", Targets: []string{"project", "target"}},
		},
		{
			name: "variable target range",
			raw:  "org:project/a/b",
			opts: util.ParseOptions{AllowImplicitOrg: false, MinTargets: 1, MaxTargets: 2},
			want: &util.Path{Organization: "org", Project: "project", Targets: []string{"a", "b"}},
		},
		{
			name:    "too many targets for variable range",
			raw:     "org:project/a/b/c",
			opts:    util.ParseOptions{AllowImplicitOrg: false, MinTargets: 1, MaxTargets: 2},
			wantErr: "expected 1-2 targets, got 3",
		},
		{
			name:    "too many non-colon segments for variable range",
			raw:     "a/b/c/d",
			opts:    util.ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 2},
			wantErr: "expected 1-2 targets, got 3",
		},
		{
			name:    "too few targets",
			raw:     "org:project",
			opts:    util.ParseOptions{AllowImplicitOrg: false, MinTargets: 1, MaxTargets: 2},
			wantErr: "expected 1-2 targets, got 0",
		},
		{
			name:    "project required with empty input",
			raw:     "",
			opts:    util.ParseOptions{AllowImplicitOrg: true, RequireProject: true, DisallowTargets: true},
			wantErr: "project is required",
		},
		{
			name:    "project required with no-project marker",
			raw:     "/target",
			opts:    util.ParseOptions{AllowImplicitOrg: true, RequireProject: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "project is required",
		},
		{
			name:    "targets disallowed with project",
			raw:     "org:project/extra",
			opts:    util.ParseOptions{AllowImplicitOrg: true, DisallowTargets: true},
			wantErr: "targets are not allowed",
		},
		{
			name:    "targets disallowed with no-project marker",
			raw:     "/extra",
			opts:    util.ParseOptions{AllowImplicitOrg: true, DisallowTargets: true},
			wantErr: "targets are not allowed",
		},
		{
			name:    "legacy form with targets disallowed",
			raw:     "org/project",
			opts:    util.ParseOptions{AllowImplicitOrg: true, DisallowTargets: true},
			wantErr: "legacy ORGANIZATION/... form is not supported, use ORG: syntax",
		},
		{
			name:    "project disallowed without marker",
			raw:     "pool/agent",
			opts:    util.ParseOptions{AllowImplicitOrg: true, DisallowProject: true, MinTargets: 2, MaxTargets: 2},
			wantErr: "project is not allowed, use the / no-project marker",
		},
		{
			name: "project disallowed with marker",
			raw:  "/pool/agent",
			opts: util.ParseOptions{AllowImplicitOrg: true, DisallowProject: true, MinTargets: 2, MaxTargets: 2},
			want: &util.Path{Organization: "default-org", Targets: []string{"pool", "agent"}},
		},
		{
			name: "project disallowed with explicit organization and marker",
			raw:  "org:/pool/agent",
			opts: util.ParseOptions{AllowImplicitOrg: true, DisallowProject: true, MinTargets: 2, MaxTargets: 2},
			want: &util.Path{Organization: "org", Targets: []string{"pool", "agent"}},
		},
		{
			name:    "project disallowed with explicit organization without marker",
			raw:     "org:pool/agent",
			opts:    util.ParseOptions{AllowImplicitOrg: true, DisallowProject: true, MinTargets: 2, MaxTargets: 2},
			wantErr: "project is not allowed, use the / no-project marker",
		},
		{
			name:    "empty segment",
			raw:     "org:/target/",
			opts:    util.ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "contains empty segment",
		},
		{
			name:    "whitespace segment",
			raw:     "org:project/ /extra",
			opts:    util.ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "contains empty segment",
		},
		{
			name:    "whitespace between colon and marker",
			raw:     "org: /target",
			opts:    util.ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "contains empty segment",
		},
		{
			name:    "whitespace only input without implicit organization",
			raw:     "   ",
			opts:    util.ParseOptions{AllowImplicitOrg: false},
			wantErr: "explicit organization is required, use ORG: syntax",
		},
		{
			name: "whitespace only input with implicit organization",
			raw:  "   ",
			opts: util.ParseOptions{AllowImplicitOrg: true, MinTargets: 0, MaxTargets: 0},
			want: &util.Path{Organization: "default-org"},
		},
		{
			name:    "multiple colons",
			raw:     "a:b:c",
			opts:    util.ParseOptions{AllowImplicitOrg: true},
			wantErr: "contains multiple colons",
		},
		{
			name:    "misplaced colon",
			raw:     "a/b:c",
			opts:    util.ParseOptions{AllowImplicitOrg: true},
			wantErr: "colon must directly follow the organization",
		},
		{
			name:    "empty organization prefix",
			raw:     ":project",
			opts:    util.ParseOptions{AllowImplicitOrg: true},
			wantErr: "organization must not be empty",
		},
		{
			name:    "no default organization configured",
			raw:     "/target",
			ctx:     emptyOrgCtx(t),
			opts:    util.ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "no organization specified and no default organization configured",
		},
		{
			name: "disallowed organization with project target",
			raw:  "project/group",
			opts: util.ParseOptions{DisallowOrganization: true, RequireProject: true, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Project: "project", Targets: []string{"group"}},
		},
		{
			name: "disallowed organization with no-project target",
			raw:  "/group",
			opts: util.ParseOptions{DisallowOrganization: true, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Targets: []string{"group"}},
		},
		{
			name:    "disallowed organization rejects explicit organization",
			raw:     "myorg:project/group",
			opts:    util.ParseOptions{DisallowOrganization: true, RequireProject: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "organization is not allowed",
		},
		{
			name:    "disallowed organization rejects legacy organization form",
			raw:     "myorg/project/group",
			opts:    util.ParseOptions{DisallowOrganization: true, RequireProject: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "organization is not allowed",
		},
		{
			name: "disallowed organization skips default organization lookup",
			raw:  "project/group",
			ctx:  emptyOrgCtx(t),
			opts: util.ParseOptions{DisallowOrganization: true, RequireProject: true, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Project: "project", Targets: []string{"group"}},
		},
		{
			name: "disallowed organization with bare project input",
			raw:  "project",
			ctx:  emptyOrgCtx(t),
			opts: util.ParseOptions{DisallowOrganization: true, RequireProject: true, DisallowTargets: true},
			want: &util.Path{Project: "project"},
		},
		{
			name: "bare target without organization",
			raw:  "5678",
			opts: util.ParseOptions{DisallowOrganization: true, AllowBareTargets: true, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Targets: []string{"5678"}},
		},
		{
			name: "bare target with default organization",
			raw:  "5678",
			opts: util.ParseOptions{AllowImplicitOrg: true, AllowBareTargets: true, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Organization: "default-org", Targets: []string{"5678"}},
		},
		{
			name:    "bare target rejects explicit organization",
			raw:     "myorg:5678",
			opts:    util.ParseOptions{DisallowOrganization: true, AllowBareTargets: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "organization is not allowed",
		},
		{
			name: "bare target marker form still accepted",
			raw:  "/5678",
			opts: util.ParseOptions{DisallowOrganization: true, AllowBareTargets: true, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Targets: []string{"5678"}},
		},
		{
			name: "project target still project first with bare targets",
			raw:  "Contoso/5678",
			opts: util.ParseOptions{DisallowOrganization: true, AllowBareTargets: true, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Project: "Contoso", Targets: []string{"5678"}},
		},
		{
			name: "two segments are not bare targets",
			raw:  "a/b",
			opts: util.ParseOptions{DisallowOrganization: true, AllowBareTargets: true, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Project: "a", Targets: []string{"b"}},
		},
		{
			name:    "bare target with too many targets",
			raw:     "a/b/c",
			opts:    util.ParseOptions{DisallowOrganization: true, AllowBareTargets: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "organization is not allowed",
		},
		{
			name:    "bare target too few targets",
			raw:     "",
			opts:    util.ParseOptions{DisallowOrganization: true, AllowBareTargets: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "expected exactly 1 targets, got 0",
		},
		{
			name: "bare target without organization constraints",
			raw:  "5678",
			opts: util.ParseOptions{DisallowOrganization: true, AllowBareTargets: true},
			want: &util.Path{Targets: []string{"5678"}},
		},
		{
			name: "disallowed organization and project with no-project target",
			raw:  "/group",
			opts: util.ParseOptions{DisallowOrganization: true, DisallowProject: true, MinTargets: 1, MaxTargets: 1},
			want: &util.Path{Targets: []string{"group"}},
		},
		{
			name:    "disallowed organization and project rejects project-first form",
			raw:     "project/group",
			opts:    util.ParseOptions{DisallowOrganization: true, DisallowProject: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "project is not allowed, use the / no-project marker",
		},
		{
			name:    "disallowed organization rejects explicit organization with no-project marker",
			raw:     "myorg:/group",
			opts:    util.ParseOptions{DisallowOrganization: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "organization is not allowed",
		},
		{
			name:    "disallowed organization rejects organization-only input",
			raw:     "myorg:",
			opts:    util.ParseOptions{DisallowOrganization: true, DisallowTargets: true},
			wantErr: "organization is not allowed",
		},
		{
			name:    "disallowed organization requires project with no-project marker",
			raw:     "/group",
			opts:    util.ParseOptions{DisallowOrganization: true, RequireProject: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "project is required",
		},
		{
			name:    "disallowed organization empty input requires project",
			raw:     "",
			opts:    util.ParseOptions{DisallowOrganization: true, RequireProject: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "project is required",
		},
		{
			name: "disallowed organization empty input with no constraints",
			raw:  "",
			opts: util.ParseOptions{DisallowOrganization: true},
			want: &util.Path{},
		},
		{
			name:    "disallowed organization too few targets",
			raw:     "project",
			opts:    util.ParseOptions{DisallowOrganization: true, RequireProject: true, MinTargets: 1, MaxTargets: 1},
			wantErr: "expected exactly 1 targets, got 0",
		},
	}

	for i := range tests {
		if tests[i].ctx == nil {
			tests[i].ctx = ctx
		}
		runParseCase(t, tests[i], func(ctx util.CmdContext, raw string) (*util.Path, error) {
			return util.Parse(ctx, raw, tests[i].opts)
		})
	}
}

func TestParse_DisallowOrganizationNilContext(t *testing.T) {
	got, err := util.Parse(nil, "project/group", util.ParseOptions{
		DisallowOrganization: true,
		RequireProject:       true,
		MinTargets:           1,
		MaxTargets:           1,
	})
	require.NoError(t, err)
	assert.Equal(t, &util.Path{Project: "project", Targets: []string{"group"}}, got)
}

func TestParseInvalidOptions(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		opts    util.ParseOptions
		want    *util.Path
		wantErr string
	}{
		{
			name:    "negative min targets",
			raw:     "org:/a/b",
			opts:    util.ParseOptions{MinTargets: -1},
			wantErr: "target counts must not be negative",
		},
		{
			name:    "negative max targets",
			raw:     "org:/a/b",
			opts:    util.ParseOptions{MaxTargets: -1},
			wantErr: "target counts must not be negative",
		},
		{
			name:    "max targets below min targets",
			raw:     "org:/a/b",
			opts:    util.ParseOptions{MinTargets: 3, MaxTargets: 2},
			wantErr: "target range [3,2] is not satisfiable",
		},
		{
			name:    "disallowed targets with min targets",
			raw:     "org:/a/b",
			opts:    util.ParseOptions{DisallowTargets: true, MinTargets: 1},
			wantErr: "targets are disallowed but min targets is 1",
		},
		{
			name:    "disallowed targets with max targets",
			raw:     "org:/a/b",
			opts:    util.ParseOptions{DisallowTargets: true, MaxTargets: 2},
			wantErr: "targets are disallowed but max targets is 2",
		},
		{
			name:    "project required and disallowed",
			raw:     "org:/a/b",
			opts:    util.ParseOptions{RequireProject: true, DisallowProject: true},
			wantErr: "project cannot be required and disallowed at the same time",
		},
		{
			name:    "organization disallowed with implicit organization",
			raw:     "org:/a/b",
			opts:    util.ParseOptions{AllowImplicitOrg: true, DisallowOrganization: true},
			wantErr: "organization cannot be disallowed when the implicit organization is allowed",
		},
		{
			name:    "bare targets with required project",
			raw:     "a/b",
			opts:    util.ParseOptions{AllowBareTargets: true, RequireProject: true},
			wantErr: "bare targets cannot be combined with a required project",
		},
		{
			name:    "bare targets with disallowed project",
			raw:     "/a/b",
			opts:    util.ParseOptions{AllowBareTargets: true, DisallowProject: true},
			wantErr: "bare targets cannot be combined with a disallowed project",
		},
		{
			name:    "bare targets with disallowed targets",
			raw:     "a/b",
			opts:    util.ParseOptions{AllowBareTargets: true, DisallowTargets: true},
			wantErr: "bare targets cannot be combined with disallowed targets",
		},
		{
			name: "disallowed targets with zero bounds is valid",
			raw:  "org:",
			opts: util.ParseOptions{AllowImplicitOrg: true, DisallowTargets: true},
			want: &util.Path{},
		},
		{
			name: "unbounded targets with min targets is valid",
			raw:  "org:/a/b",
			opts: util.ParseOptions{AllowImplicitOrg: true, MinTargets: 2},
			want: &util.Path{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := util.Parse(defaultOrgCtx(t), tt.raw, tt.opts)
			if tt.want != nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
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

func TestResolveScopeDescriptor_GetProjectError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCtx := mocks.NewMockCmdContext(ctrl)
	mockClientFactory := mocks.NewMockClientFactory(ctrl)
	mockCoreClient := mocks.NewMockCoreClient(ctrl)

	mockCtx.EXPECT().ClientFactory().Return(mockClientFactory).AnyTimes()
	mockCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mockClientFactory.EXPECT().
		Core(gomock.Any(), "org").
		Return(mockCoreClient, nil)
	mockCoreClient.EXPECT().
		GetProject(gomock.Any(), gomock.AssignableToTypeOf(core.GetProjectArgs{})).
		Return(nil, errors.New("boom"))

	descriptor, projectID, err := util.ResolveScopeDescriptor(mockCtx, "org", "project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get project")
	assert.Nil(t, descriptor)
	assert.Nil(t, projectID)
}

func TestResolveScopeDescriptor_MissingProjectID(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCtx := mocks.NewMockCmdContext(ctrl)
	mockClientFactory := mocks.NewMockClientFactory(ctrl)
	mockCoreClient := mocks.NewMockCoreClient(ctrl)

	mockCtx.EXPECT().ClientFactory().Return(mockClientFactory).AnyTimes()
	mockCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mockClientFactory.EXPECT().
		Core(gomock.Any(), "org").
		Return(mockCoreClient, nil)
	mockCoreClient.EXPECT().
		GetProject(gomock.Any(), gomock.AssignableToTypeOf(core.GetProjectArgs{})).
		Return(&core.TeamProject{}, nil)

	descriptor, projectID, err := util.ResolveScopeDescriptor(mockCtx, "org", "project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project storage key is missing")
	assert.Nil(t, descriptor)
	assert.Nil(t, projectID)
}

func TestResolveScopeDescriptor_GraphClientError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCtx := mocks.NewMockCmdContext(ctrl)
	mockClientFactory := mocks.NewMockClientFactory(ctrl)
	mockCoreClient := mocks.NewMockCoreClient(ctrl)

	mockCtx.EXPECT().ClientFactory().Return(mockClientFactory).AnyTimes()
	mockCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	projectID := uuid.New()
	mockClientFactory.EXPECT().
		Core(gomock.Any(), "org").
		Return(mockCoreClient, nil)
	mockCoreClient.EXPECT().
		GetProject(gomock.Any(), gomock.AssignableToTypeOf(core.GetProjectArgs{})).
		Return(&core.TeamProject{Id: types.ToPtr(projectID)}, nil)
	mockClientFactory.EXPECT().
		Graph(gomock.Any(), "org").
		Return(nil, errors.New("boom"))

	descriptor, projectIDPtr, err := util.ResolveScopeDescriptor(mockCtx, "org", "project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create graph client")
	assert.Nil(t, descriptor)
	assert.Nil(t, projectIDPtr)
}

func TestResolveScopeDescriptor_EmptyDescriptor(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCtx := mocks.NewMockCmdContext(ctrl)
	mockClientFactory := mocks.NewMockClientFactory(ctrl)
	mockCoreClient := mocks.NewMockCoreClient(ctrl)
	mockGraphClient := mocks.NewMockGraphClient(ctrl)

	mockCtx.EXPECT().ClientFactory().Return(mockClientFactory).AnyTimes()
	mockCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	projectID := uuid.New()
	mockClientFactory.EXPECT().
		Core(gomock.Any(), "org").
		Return(mockCoreClient, nil)
	mockCoreClient.EXPECT().
		GetProject(gomock.Any(), gomock.AssignableToTypeOf(core.GetProjectArgs{})).
		Return(&core.TeamProject{Id: types.ToPtr(projectID)}, nil)
	mockClientFactory.EXPECT().
		Graph(gomock.Any(), "org").
		Return(mockGraphClient, nil)
	empty := ""
	mockGraphClient.EXPECT().
		GetDescriptor(gomock.Any(), gomock.AssignableToTypeOf(graph.GetDescriptorArgs{})).
		Return(&graph.GraphDescriptorResult{Value: &empty}, nil)

	descriptor, projectIDPtr, err := util.ResolveScopeDescriptor(mockCtx, "org", "project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project descriptor is empty")
	assert.Nil(t, descriptor)
	assert.Nil(t, projectIDPtr)
}
