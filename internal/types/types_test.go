package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupEnum_Match(t *testing.T) {
	t.Parallel()

	got, ok := LookupEnum("BeTa", []string{"alpha", "beta", "gamma"})

	require.True(t, ok)
	assert.Equal(t, "beta", got)
}

func TestLookupEnum_NoMatch(t *testing.T) {
	t.Parallel()

	got, ok := LookupEnum("delta", []string{"alpha", "beta", "gamma"})

	assert.False(t, ok)
	assert.Equal(t, "", got)
}

func TestLookupEnum_NamedStringType(t *testing.T) {
	t.Parallel()

	type enum string

	got, ok := LookupEnum("SECOND", []enum{"first", "second"})

	require.True(t, ok)
	assert.Equal(t, enum("second"), got)
}
