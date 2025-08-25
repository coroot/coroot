package utils

func Ptr[T any](v T) *T {
	return &v
}

func Uniq[T comparable](s []T) []T {
	m := map[T]struct{}{}
	for _, v := range s {
		m[v] = struct{}{}
	}
	res := make([]T, 0, len(m))
	for v := range m {
		res = append(res, v)
	}
	return res
}
