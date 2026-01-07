package types

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnique_RemovesDuplicatesPreservesOrder(t *testing.T) {
	input := []int{3, 1, 3, 2, 1, 2, 3}
	assert.Equal(t, []int{3, 1, 2}, Unique(input))
}

func TestUnique_Empty(t *testing.T) {
	assert.Equal(t, []string{}, Unique([]string{}))
}

func TestUnique_Nil(t *testing.T) {
	assert.Equal(t, []string{}, Unique[string](nil))
}

func TestUniqueComparable_CaseInsensitive(t *testing.T) {
	input := []string{"Alpha", "beta", "ALPHA", "BETA", "gamma"}
	assert.Equal(t, []string{"Alpha", "beta", "gamma"}, UniqueComparable(input, strings.ToLower))
}

func TestUniqueComparable_Nil(t *testing.T) {
	assert.Equal(t, []string{}, UniqueComparable(nil, strings.ToLower))
}

func TestUniqueComparable_UsesKeyFunction(t *testing.T) {
	type entry struct {
		ID   int
		Name string
	}

	input := []entry{
		{ID: 1, Name: "one"},
		{ID: 2, Name: "two"},
		{ID: 1, Name: "uno"},
		{ID: 3, Name: "three"},
		{ID: 2, Name: "dos"},
	}

	unique := UniqueComparable(input, func(e entry) int { return e.ID })
	assert.Equal(t, []entry{
		{ID: 1, Name: "one"},
		{ID: 2, Name: "two"},
		{ID: 3, Name: "three"},
	}, unique)
}

func TestUniqueFunc_RemovesDuplicatesUsingComparator(t *testing.T) {
	input := []string{"A", "b", "a", "B", "c"}
	unique := UniqueFunc(input, func(a, b string) bool { return strings.EqualFold(a, b) })
	assert.Equal(t, []string{"A", "b", "c"}, unique)
}

func TestUniqueFunc_Empty(t *testing.T) {
	assert.Equal(t, []int{}, UniqueFunc([]int{}, func(a, b int) bool { return a == b }))
}

func TestUniqueFunc_Nil(t *testing.T) {
	assert.Equal(t, []int{}, UniqueFunc(nil, func(a, b int) bool { return a == b }))
}
