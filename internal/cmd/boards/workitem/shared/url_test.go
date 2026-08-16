package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseWorkItemURL covers the best-effort URL parser used for relation
// fallback. It moved here from the relation shared package when the work-item
// URL helpers were consolidated.
func TestParseWorkItemURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		raw         string
		wantOrg     string
		wantProject string
	}{
		{
			name:        "dev.azure with project",
			raw:         "https://dev.azure.com/myorg/Contoso/_apis/wit/workItems/77",
			wantOrg:     "myorg",
			wantProject: "Contoso",
		},
		{
			name:        "dev.azure without _apis falls back to first segment",
			raw:         "https://dev.azure.com/myorg/Contoso",
			wantOrg:     "myorg",
			wantProject: "myorg",
		},
		{
			name:        "visualstudio with project",
			raw:         "https://myorg.visualstudio.com/Contoso/_apis/wit/workItems/77",
			wantOrg:     "myorg",
			wantProject: "Contoso",
		},
		{
			name:        "visualstudio org-only",
			raw:         "https://myorg.visualstudio.com",
			wantOrg:     "myorg",
			wantProject: "",
		},
		{
			name:        "uppercase host",
			raw:         "https://Dev.Azure.com/MyOrg/Contoso/_apis/wit/workItems/77",
			wantOrg:     "myorg",
			wantProject: "Contoso",
		},
		{
			name:        "non-azure host",
			raw:         "https://example.com/1",
			wantOrg:     "example.com",
			wantProject: "1",
		},
		{
			name:        "malformed",
			raw:         "not a url",
			wantOrg:     "",
			wantProject: "",
		},
		{
			name:        "empty",
			raw:         "",
			wantOrg:     "",
			wantProject: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotOrg, gotProject := ParseWorkItemURL(tt.raw)
			assert.Equal(t, tt.wantOrg, gotOrg)
			assert.Equal(t, tt.wantProject, gotProject)
		})
	}
}
