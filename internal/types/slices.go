package types

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

func Unique[T comparable](items []T) []T {
	return uniqueByEq(items, func(s1 T, s2 T) bool {
		return s1 == s2
	})
}

func UniqueComparable[T any, C comparable](items []T, cmp func(T) C) []T {
	return uniqueByEq(items, func(s1 T, s2 T) bool {
		return cmp(s1) == cmp(s2)
	})
}

func UniqueFunc[T any](items []T, cmp func(T, T) bool) []T {
	return uniqueByEq(items, cmp)
}

func uniqueByEq[T any](items []T, eq func(T, T) bool) []T {
	if len(items) == 0 {
		return []T{}
	}

	unique := make([]T, 0, len(items))
	for _, item := range items {
		duplicate := false
		for i := range unique {
			if eq(unique[i], item) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		unique = append(unique, item)
	}
	return unique
}

func MapSlice[S any, T any](items []S, mapper func(S) T) []T {
	if len(items) == 0 {
		return []T{}
	}
	result := make([]T, len(items))
	for i, item := range items {
		result[i] = mapper(item)
	}
	return result
}

func MapSlicePtr[S any, T any](items *[]S, mapper func(S) T) []T {
	if items == nil {
		return []T{}
	}
	return MapSlice(*items, mapper)
}

// func UniqueErrors[T error](items []T) []T {
// 	return UniqueFunc(items, func(e1 T, e2 T) bool {
// 		return e1.Error() == e2.Error()
// 	})
// }

func GetValueOrDefault[T any](items []T, index int, defaultValue T) T {
	if index < 0 || index >= len(items) {
		return defaultValue
	}
	return items[index]
}

// CompareUnorderedSlices compares two slices as unordered multisets (i.e., checks if they contain the same elements
// with the same frequencies, ignoring order). It requires elements of type T to be comparable (e.g., ints, strings).
//
// Parameters:
//   - a, b: The slices to compare.
//
// Returns true if the slices contain the same elements with the same multiplicities, false otherwise.
//
// Example usage:
//
//	a := []int{1, 2, 3}
//	b := []int{3, 1, 2}
//	result := CompareUnorderedSlices(a, b) // true
//
// Note: This function has average O(n) time complexity due to the use of a hash map for counting frequencies.
func CompareUnorderedSlices[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[T]int)
	for _, v := range a {
		counts[v]++
	}
	for _, v := range b {
		if counts[v] == 0 {
			return false
		}
		counts[v]--
	}
	return true
}

// CompareUnorderedSlicesKey compares two slices as unordered multisets (i.e., checks if they contain the same elements
// with the same frequencies, ignoring order). It uses a provided key function to determine equivalence between elements.
//
// Parameters:
//   - a, b: The slices to compare.
//   - key: A function that maps each element of type T to a comparable key of type K. Two elements are considered
//     equivalent if key(x) == key(y). This allows custom equality, such as case-insensitive comparison for strings
//     (e.g., key = strings.ToLower).
//
// Returns true if the slices are equivalent under the key function, false otherwise.
//
// Example usage for case-insensitive string comparison:
//
//	a := []string{"Apple", "banana"}
//	b := []string{"apple", "Banana"}
//	result := CompareUnorderedSlicesKey(a, b, strings.ToLower) // true
//
// Note: This function has average O(n) time complexity assuming good key distribution for hashing in the map.
func CompareUnorderedSlicesKey[T any, K comparable](a, b []T, key func(T) K) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[K]int)
	for _, v := range a {
		k := key(v)
		counts[k]++
	}
	for _, v := range b {
		k := key(v)
		if counts[k] == 0 {
			return false
		}
		counts[k]--
	}
	return true
}
