package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPRRegex(t *testing.T) {
	type resultData struct {
		Org  string
		Proj string
		Repo string
		PrID int
	}
	tests := []struct {
		S          string
		wantResult resultData
		wantErr    bool
	}{
		{
			S: "my org/proj   skjfsdfkj898595838/repo sdfuia9q3459--:89",
			wantResult: resultData{
				Org:  "my org",
				Proj: "proj   skjfsdfkj898595838",
				Repo: "repo sdfuia9q3459--",
				PrID: 89,
			},
		},
		{
			S: "6477",
			wantResult: resultData{
				PrID: 6477,
			},
		},
		{
			S: "#7843",
			wantResult: resultData{
				PrID: 7843,
			},
		},
		{
			S:       "#78s43",
			wantErr: true,
		},
	}

	for _, tst := range tests {
		org, prj, repo, prID, err := parseSelector(tst.S)
		if tst.wantErr {
		} else {
		}
		require.Condition(t, func() bool {
			return (tst.wantErr && err != nil) || (!tst.wantErr && err == nil)
		})
		assert.Equal(t, tst.wantResult.Org, org)
		assert.Equal(t, tst.wantResult.Proj, prj)
		assert.Equal(t, tst.wantResult.Repo, repo)
		assert.Equal(t, tst.wantResult.PrID, prID)
	}
}
