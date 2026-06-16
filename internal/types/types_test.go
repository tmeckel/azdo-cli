package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnumLookup(t *testing.T) {
	t.Parallel()

	lookup := EnumLookup[string]{
		"none":   "none",
		"manage": "manage",
		"use":    "use",
	}

	t.Run("keys are sorted", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, []string{"manage", "none", "use"}, lookup.Keys())
	})

	t.Run("get value", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			input  string
			want   string
			wantOK bool
		}{
			{name: "exact", input: "manage", want: "manage", wantOK: true},
			{name: "trimmed and lowercased", input: " Use ", want: "use", wantOK: true},
			{name: "invalid", input: "admin", wantOK: false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, ok := lookup.GetValue(tt.input)
				assert.Equal(t, tt.wantOK, ok)
				if !tt.wantOK {
					assert.Equal(t, "", got)
					return
				}
				assert.Equal(t, tt.want, got)
			})
		}
	})

	t.Run("get value ptr", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			input  *string
			want   *string
			wantOK bool
		}{
			{name: "nil", input: nil, want: nil, wantOK: true},
			{name: "valid", input: ToPtr(" Manage "), want: ToPtr("manage"), wantOK: true},
			{name: "invalid", input: ToPtr("admin"), want: nil, wantOK: false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, ok := lookup.GetValuePtr(tt.input)
				assert.Equal(t, tt.wantOK, ok)
				if tt.want == nil {
					assert.Nil(t, got)
					return
				}

				require.NotNil(t, got)
				assert.Equal(t, *tt.want, *got)
			})
		}
	})
}
