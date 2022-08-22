package utils

import (
	"encoding/json"
	"sort"
)

type StringSet struct {
	m map[string]struct{}
}

func NewStringSet(items ...string) *StringSet {
	ss := &StringSet{m: map[string]struct{}{}}
	for _, s := range items {
		ss.m[s] = struct{}{}
	}
	return ss
}

func (ss *StringSet) Add(items ...string) {
	for _, s := range items {
		if s == "" {
			continue
		}
		if ss.m == nil {
			ss.m = map[string]struct{}{}
		}
		ss.m[s] = struct{}{}
	}
}

func (ss *StringSet) Has(s string) bool {
	if ss.m == nil {
		return false
	}
	_, ok := ss.m[s]
	return ok
}

func (ss *StringSet) Delete(s string) {
	delete(ss.m, s)
}

func (ss *StringSet) Len() int {
	return len(ss.m)
}

func (ss *StringSet) Items() []string {
	var res []string
	for s := range ss.m {
		res = append(res, s)
	}
	sort.Strings(res)
	return res
}

func (ss *StringSet) MarshalJSON() ([]byte, error) {
	sl := make([]string, 0, len(ss.m))
	for el, _ := range ss.m {
		sl = append(sl, el)
	}
	sort.Strings(sl)
	return json.Marshal(sl)
}
