package shared

import (
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/types"
)

func TestParseBits(t *testing.T) {
	actions := []security.ActionDefinition{
		{
			Bit:         types.ToPtr(1),
			Name:        types.ToPtr("Read"),
			DisplayName: types.ToPtr("Read"),
		},
		{
			Bit:         types.ToPtr(2),
			Name:        types.ToPtr("Edit"),
			DisplayName: types.ToPtr("Modify"),
		},
		{
			Bit:         types.ToPtr(4),
			Name:        types.ToPtr("Contribute"),
			DisplayName: types.ToPtr("Contribute"),
		},
	}

	tests := []struct {
		name    string
		input   []string
		want    int
		wantErr bool
	}{
		{
			name:  "hexadecimal value",
			input: []string{"0x4"},
			want:  4,
		},
		{
			name:  "decimal value",
			input: []string{"2"},
			want:  2,
		},
		{
			name:  "textual name",
			input: []string{"Read"},
			want:  1,
		},
		{
			name:  "display name",
			input: []string{"Modify"},
			want:  2,
		},
		{
			name:  "combined values",
			input: []string{"Read", "0x2"},
			want:  3,
		},
		{
			name:    "invalid token",
			input:   []string{"UnknownPermission"},
			wantErr: true,
		},
		{
			name:    "unknown expression with hex",
			input:   []string{"Unknown (0x4)"},
			wantErr: true,
		},
		{
			name:    "unknown bit",
			input:   []string{"0x8"},
			wantErr: true,
		},
		{
			name:  "empty",
			input: []string{},
			want:  0,
		},
		{
			name:    "combined invalid values",
			input:   []string{"Read", "0x2", "0x8"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePermissionBits(actions, tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
