package utils

import (
	"cmp"
	"sort"
)

func Ptr[T any](v T) *T {
	return &v
}

func SortSlice[T cmp.Ordered](s []T) {
	sort.Slice(s, func(i, j int) bool {
		return s[i] < s[j]
	})
}
