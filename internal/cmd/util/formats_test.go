package util

import (
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tmeckel/azdo-cli/internal/types"
)

func TestFormatTimePtr(t *testing.T) {
	now := azuredevops.Time{Time: time.Date(2024, 6, 5, 14, 30, 0, 0, time.UTC)}
	tests := []struct {
		name string
		ts   *azuredevops.Time
		want *string
	}{
		{name: "nil", ts: nil, want: nil},
		{name: "valid time", ts: &now, want: types.ToPtr(now.AsQueryParameter())},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTimePtr(tt.ts)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, *tt.want, *got)
		})
	}
}

func TestFormatTimeShort(t *testing.T) {
	now := azuredevops.Time{Time: time.Date(2024, 6, 5, 14, 30, 0, 0, time.UTC)}
	tests := []struct {
		name string
		ts   *azuredevops.Time
		want string
	}{
		{name: "nil", ts: nil, want: ""},
		{name: "zero value", ts: &azuredevops.Time{}, want: ""},
		{name: "valid time", ts: &now, want: "2024-06-05 14:30:00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FormatTimeShort(tt.ts))
		})
	}
}
