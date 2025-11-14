package extensions

import (
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/stretchr/testify/assert"
	"github.com/tmeckel/azdo-cli/internal/types"
)

func Test_determineIdentitySearchFilters(t *testing.T) {
	testCases := []struct {
		name   string
		member string
		want   []string
	}{
		{
			name:   "email",
			member: "user@example.com",
			want:   []string{"AccountName", "MailAddress"},
		},
		{
			name:   "account name with slash",
			member: "DOMAIN\\user",
			want:   []string{"AccountName"},
		},
		{
			name:   "name with space",
			member: "First Last",
			want:   []string{"General", "AccountName", "DirectoryAlias", "LocalGroupName"},
		},
		{
			name:   "mixed content",
			member: "user@domain\\other",
			want:   []string{"AccountName", "MailAddress"},
		},
		{
			name:   "trimmed input is used",
			member: "  alias  ",
			want:   []string{"DirectoryAlias", "AccountName", "General", "LocalGroupName"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := determineIdentitySearchFilters(tc.member)
			assert.True(t, types.CompareUnorderedSlices(tc.want, got), "want:%+v != got:%+v", tc.want, got)
		})
	}
}

func Test_memberSubjectKind(t *testing.T) {
	t.Run("returns group when IsContainer true", func(t *testing.T) {
		kind := memberSubjectKind(identity.Identity{IsContainer: types.ToPtr(true)})
		assert.Equal(t, "Group", kind)
	})

	t.Run("returns user when IsContainer false", func(t *testing.T) {
		kind := memberSubjectKind(identity.Identity{IsContainer: types.ToPtr(false)})
		assert.Equal(t, "User", kind)
	})

	t.Run("returns user when IsContainer nil", func(t *testing.T) {
		kind := memberSubjectKind(identity.Identity{})
		assert.Equal(t, "User", kind)
	})
}
