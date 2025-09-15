package utils

import (
	"encoding/json"
	"sort"

	"golang.org/x/exp/maps"
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
	if ss == nil || ss.m == nil {
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
	if ss == nil {
		return []string{}
	}
	res := maps.Keys(ss.m)
	sort.Strings(res)
	return res
}

func (ss *StringSet) GetFirst() string {
	if ss.Len() == 0 {
		return ""
	}
	return ss.Items()[0]
}

func (ss *StringSet) MarshalJSON() ([]byte, error) {
	return json.Marshal(ss.Items())
}

func (ss *StringSet) UnmarshalJSON(data []byte) error {
	var items []string
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	ss.Add(items...)
	return nil
}
