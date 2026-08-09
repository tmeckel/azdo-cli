package shared

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

func TestParseSubjectTarget(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		ctx     util.CmdContext
		want    *SubjectTarget
		wantErr string
	}{
		{
			name:  "empty input uses default organization",
			input: "",
			ctx:   defaultOrgCtx(t),
			want: &SubjectTarget{
				Path: util.Path{Organization: "default-org"},
			},
		},
		{
			name:  "organization only with marker",
			input: "myorg:/",
			ctx:   defaultOrgCtx(t),
			want: &SubjectTarget{
				Path: util.Path{Organization: "myorg"},
			},
		},
		{
			name:    "organization only without marker is rejected",
			input:   "myorg:",
			ctx:     defaultOrgCtx(t),
			wantErr: "use ORG:/ to specify an organization without a project or targets",
		},
		{
			name:  "bare organization is a project",
			input: "myorg",
			ctx:   defaultOrgCtx(t),
			want: &SubjectTarget{
				Path: util.Path{Organization: "default-org", Project: "myorg"},
			},
		},
		{
			name:  "organization level subject with default organization",
			input: "/user@example.com",
			ctx:   defaultOrgCtx(t),
			want: &SubjectTarget{
				Path:    util.Path{Organization: "default-org", Targets: []string{"user@example.com"}},
				Subject: "user@example.com",
			},
		},
		{
			name:  "organization level subject with explicit organization",
			input: "myorg:/user@example.com",
			ctx:   nil,
			want: &SubjectTarget{
				Path:    util.Path{Organization: "myorg", Targets: []string{"user@example.com"}},
				Subject: "user@example.com",
			},
		},
		{
			name:  "project subject with default organization",
			input: "myproject/contoso@example.com",
			ctx:   defaultOrgCtx(t),
			want: &SubjectTarget{
				Path:    util.Path{Organization: "default-org", Project: "myproject", Targets: []string{"contoso@example.com"}},
				Subject: "contoso@example.com",
			},
		},
		{
			name:  "project subject with explicit organization",
			input: "myorg:myproject/contoso@example.com",
			ctx:   nil,
			want: &SubjectTarget{
				Path:    util.Path{Organization: "myorg", Project: "myproject", Targets: []string{"contoso@example.com"}},
				Subject: "contoso@example.com",
			},
		},
		{
			name:  "legacy two segment subject is project first",
			input: "myorg/user@example.com",
			ctx:   defaultOrgCtx(t),
			want: &SubjectTarget{
				Path:    util.Path{Organization: "default-org", Project: "myorg", Targets: []string{"user@example.com"}},
				Subject: "user@example.com",
			},
		},
		{
			name:  "no project marker without subject",
			input: "/",
			ctx:   defaultOrgCtx(t),
			want: &SubjectTarget{
				Path: util.Path{Organization: "default-org"},
			},
		},
		{
			name:  "whitespace is trimmed",
			input: "  myorg:/user@example.com  ",
			ctx:   nil,
			want: &SubjectTarget{
				Path:    util.Path{Organization: "myorg", Targets: []string{"user@example.com"}},
				Subject: "user@example.com",
			},
		},
		{
			name:    "more than one subject is rejected",
			input:   "myorg:/first/second",
			ctx:     nil,
			wantErr: "expected 0-1 targets, got 2",
		},
		{
			name:    "multiple colons are rejected",
			input:   "a:b:/subject",
			ctx:     nil,
			wantErr: "contains multiple colons",
		},
		{
			name:    "empty organization prefix is rejected",
			input:   ":/subject",
			ctx:     nil,
			wantErr: "organization must not be empty",
		},
		{
			name:    "empty trailing segment is rejected",
			input:   "myorg:/subject/",
			ctx:     nil,
			wantErr: "contains empty segment",
		},
		{
			name:    "missing default organization is an error",
			input:   "/user@example.com",
			ctx:     nil,
			wantErr: "no organization specified and no default organization configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSubjectTarget(tt.ctx, tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
