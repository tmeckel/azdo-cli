package types

func ToPtr[T any](value T) *T {
	return &value
}

func GetValue[T any](ptr *T, defaultVal T) T {
	if ptr == nil {
		return defaultVal
	}
	return *ptr
}
