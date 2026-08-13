package shared

import (
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBelongsToProject(t *testing.T) {
	t.Parallel()

	fields := map[string]interface{}{TeamProjectField: "Fabrikam"}
	tests := []struct {
		name    string
		item    *workitemtracking.WorkItem
		project string
		want    bool
	}{
		{name: "nil item", item: nil, project: "Fabrikam", want: false},
		{name: "nil fields", item: &workitemtracking.WorkItem{}, project: "Fabrikam", want: false},
		{name: "missing field", item: &workitemtracking.WorkItem{Fields: &map[string]interface{}{}}, project: "Fabrikam", want: false},
		{name: "match", item: &workitemtracking.WorkItem{Fields: &fields}, project: "Fabrikam", want: true},
		{name: "mismatch", item: &workitemtracking.WorkItem{Fields: &fields}, project: "Contoso", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, BelongsToProject(tc.item, tc.project))
		})
	}

	require.Equal(t, "System.TeamProject", TeamProjectField)
}
