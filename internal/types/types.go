package types

import (
	"cmp"
	"maps"
	"slices"
	"strings"
)

// ToPtr returns a pointer to value.
func ToPtr[T any](value T) *T {
	return &value
}

// GetValue dereferences ptr and returns the value, or defaultVal if ptr is nil.
func GetValue[T any](ptr *T, defaultVal T) T {
	if ptr == nil {
		return defaultVal
	}
	return *ptr
}

// NotZeroPtrOrNil returns a pointer to v if v is not the zero value, otherwise nil.
func NotZeroPtrOrNil[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}

// PositivePtrOrNil returns a pointer to v if v > zero value, otherwise nil.
// Useful for converting numeric flags where non-positive means "not set".
func PositivePtrOrNil[T cmp.Ordered](v T) *T {
	var zero T
	if v <= zero {
		return nil
	}
	return &v
}

type EnumLookup[T any] map[string]T

func (l EnumLookup[T]) Keys() []string {
	keys := slices.Collect(maps.Keys(l))
	slices.Sort(keys)
	return keys
}

func (l EnumLookup[T]) GetValue(value string) (T, bool) {
	v, ok := l[strings.ToLower(strings.TrimSpace(value))]
	if !ok {
		var zero T
		return zero, false
	}
	return v, true
}

func (l EnumLookup[T]) GetValuePtr(value *string) (*T, bool) {
	if value == nil {
		return nil, true
	}
	v, ok := l.GetValue(*value)
	if !ok {
		return nil, false
	}
	return ToPtr(v), true
}
