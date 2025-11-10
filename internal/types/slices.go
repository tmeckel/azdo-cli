package types

import (
	"fmt"
	"slices"
)

func FilterSlice[T comparable](slice []T, filters ...func(value T, index int) (bool, error)) ([]T, error) {
	if len(filters) == 0 {
		return slice, nil
	}
	res := []T{}
	for index, value := range slice {
		var ok bool
		var err error
		for _, f := range filters {
			ok, err = f(value, index)
			if err != nil {
				return []T{}, err
			}
			if !ok {
				break
			}
		}
		if ok {
			res = append(res, value)
		}
	}
	return res, nil
}

func UniqueByString[T fmt.Stringer](items []T) []T {
	return slices.CompactFunc(items, func(s1 T, s2 T) bool {
		return s1.String() == s2.String()
	})
}

func UniqueErrors[T error](items []T) []T {
	return slices.CompactFunc(items, func(e1 T, e2 T) bool {
		return e1.Error() == e2.Error()
	})
}
