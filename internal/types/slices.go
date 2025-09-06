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
