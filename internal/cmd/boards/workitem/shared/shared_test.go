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
		{name: "field lowercased", item: &workitemtracking.WorkItem{Fields: &map[string]interface{}{TeamProjectField: "fabrikam"}}, project: "Fabrikam", want: true},
		{name: "project uppercased", item: &workitemtracking.WorkItem{Fields: &fields}, project: "FABRIKAM", want: true},
		{name: "mixed casing both sides", item: &workitemtracking.WorkItem{Fields: &map[string]interface{}{TeamProjectField: "fAbRiKaM"}}, project: "Fabrikam", want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, BelongsToProject(tc.item, tc.project))
		})
	}

	require.Equal(t, "System.TeamProject", TeamProjectField)
}

func TestSplitUnderPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		wantOK   bool
		wantPath string
	}{
		{name: "canonical prefix", raw: "Under:Web/Payments", wantOK: true, wantPath: "Web/Payments"},
		{name: "mixed case prefix", raw: "uNdeR:Web/Payments", wantOK: true, wantPath: "Web/Payments"}, // spellchecker:disable-line
		{name: "prefix only", raw: "Under:", wantOK: true, wantPath: ""},
		{name: "bare path", raw: "Web/Payments", wantOK: false, wantPath: "Web/Payments"},
		{name: "path starting with under", raw: "Underwood", wantOK: false, wantPath: "Underwood"},
		{name: "empty", raw: "", wantOK: false, wantPath: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ok, path := SplitUnderPrefix(tc.raw)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantPath, path)
		})
	}
}

func TestNormalizePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		project string
		raw     string
		want    string
	}{
		{name: "empty input", project: "Fabrikam", raw: "", want: ""},
		{name: "whitespace only", project: "Fabrikam", raw: "   ", want: ""},
		{name: "relative slash shorthand", project: "Fabrikam", raw: "Web/Payments", want: `Fabrikam\Web\Payments`},
		{name: "relative with surrounding whitespace", project: "Fabrikam", raw: "  Web/Payments  ", want: `Fabrikam\Web\Payments`},
		{name: "rooted backslash preserved", project: "Fabrikam", raw: `Fabrikam\Area\Voice`, want: `Fabrikam\Area\Voice`},
		{name: "rooted case-insensitive preserved", project: "Fabrikam", raw: `fabrikam\Web`, want: `fabrikam\Web`},
		{name: "project name alone case-insensitive preserved", project: "Fabrikam", raw: "fabrikam", want: "fabrikam"},
		{name: "leading backslash stripped", project: "Fabrikam", raw: `\Web\Payments`, want: `Fabrikam\Web\Payments`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, NormalizePath(tc.project, tc.raw))
		})
	}
}
