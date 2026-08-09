package show

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"go.uber.org/mock/gomock"
)

// defaultOrgCtx returns a CmdContext whose configuration resolves the default
// organization to "default-org".
func defaultOrgCtx(t *testing.T) util.CmdContext {
	t.Helper()

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

func TestParseNamespaceTarget(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		ctx     util.CmdContext
		want    *util.Path
		wantErr string
	}{
		{
			name:  "namespace with default organization",
			input: "/52d39943-cb85-4d7f-8fa8-c6baac873819",
			ctx:   defaultOrgCtx(t),
			want:  &util.Path{Organization: "default-org", Targets: []string{"52d39943-cb85-4d7f-8fa8-c6baac873819"}},
		},
		{
			name:  "namespace with explicit organization",
			input: "myorg:/Project Collection",
			ctx:   nil,
			want:  &util.Path{Organization: "myorg", Targets: []string{"Project Collection"}},
		},
		{
			name:  "namespace name with whitespace is trimmed",
			input: "  myorg:/Build  ",
			ctx:   nil,
			want:  &util.Path{Organization: "myorg", Targets: []string{"Build"}},
		},
		{
			name:    "bare namespace is rejected",
			input:   "Build",
			ctx:     defaultOrgCtx(t),
			wantErr: "project is not allowed, use the / no-project marker",
		},
		{
			name:    "legacy organization slash namespace is rejected",
			input:   "myorg/Build",
			ctx:     defaultOrgCtx(t),
			wantErr: "project is not allowed, use the / no-project marker",
		},
		{
			name:    "empty input is rejected",
			input:   "",
			ctx:     defaultOrgCtx(t),
			wantErr: "expected exactly 1 targets, got 0",
		},
		{
			name:    "namespace marker without identifier is rejected",
			input:   "myorg:/",
			ctx:     nil,
			wantErr: "expected exactly 1 targets, got 0",
		},
		{
			name:    "more than one namespace is rejected",
			input:   "myorg:/first/second",
			ctx:     nil,
			wantErr: "expected exactly 1 targets, got 2",
		},
		{
			name:    "empty organization prefix is rejected",
			input:   ":/Build",
			ctx:     nil,
			wantErr: "organization must not be empty",
		},
		{
			name:    "missing default organization is an error",
			input:   "/Build",
			ctx:     nil,
			wantErr: "no organization specified and no default organization configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, identifier, err := parseNamespaceTarget(tt.ctx, tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, path)
			assert.Equal(t, tt.want.Targets[0], identifier)
		})
	}
}
