package prom

type FilterLabelsF func(name string) bool

func FilterLabelsKeepAll(name string) bool {
	return true
}

func FilterLabelsDropAll(name string) bool {
	return false
}
