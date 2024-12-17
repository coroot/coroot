package model

import (
	"sort"
	"strings"
)

type ProfileCategory string

const (
	ProfileCategoryNone   = ""
	ProfileCategoryCPU    = "cpu"
	ProfileCategoryMemory = "memory"
)

type ProfileType string

const (
	ProfileTypeEbpfCPU            ProfileType = "ebpf:cpu:nanoseconds"
	ProfileTypeGoCPU              ProfileType = "go:profile_cpu:nanoseconds"
	ProfileTypeGoHeapAllocObjects ProfileType = "go:heap_alloc_objects:count"
	ProfileTypeGoHeapAllocSpace   ProfileType = "go:heap_alloc_space:bytes"
	ProfileTypeGoHeapInuseObjects ProfileType = "go:heap_inuse_objects:count"
	ProfileTypeGoHeapInuseSpace   ProfileType = "go:heap_inuse_space:bytes"
	ProfileTypeGoGoroutines       ProfileType = "go:goroutine_goroutine:count"
	ProfileTypeGoBlockContentions ProfileType = "go:block_contentions:count"
	ProfileTypeGoBlockDelay       ProfileType = "go:block_delay:nanoseconds"
	ProfileTypeGoMutexContentions ProfileType = "go:mutex_contentions:count"
	ProfileTypeGoMutexDelay       ProfileType = "go:mutex_delay:nanoseconds"
)

type ProfileAggregation string

const (
	ProfileAggregationSum ProfileAggregation = "sum"
	ProfileAggregationAvg ProfileAggregation = "avg"
)

type ProfileMeta struct {
	Category    ProfileCategory
	Name        string
	Aggregation ProfileAggregation
	Featured    bool
	Ebpf        bool
}

type Profile struct {
	Type       ProfileType     `json:"type"`
	FlameGraph *FlameGraphNode `json:"flamegraph"`
	Diff       bool            `json:"diff"`
}

var (
	Profiles = map[ProfileType]ProfileMeta{
		ProfileTypeEbpfCPU: {
			Category:    ProfileCategoryCPU,
			Name:        "CPU (eBPF)",
			Aggregation: ProfileAggregationSum,
			Ebpf:        true,
		},
		ProfileTypeGoCPU: {
			Category:    ProfileCategoryCPU,
			Name:        "CPU",
			Aggregation: ProfileAggregationSum,
			Featured:    true,
		},
		ProfileTypeGoHeapAllocObjects: {
			Category:    ProfileCategoryMemory,
			Name:        "Memory (alloc_objects)",
			Aggregation: ProfileAggregationSum,
		},
		ProfileTypeGoHeapAllocSpace: {
			Category:    ProfileCategoryMemory,
			Name:        "Memory (alloc_space)",
			Aggregation: ProfileAggregationSum,
		},
		ProfileTypeGoHeapInuseObjects: {
			Category:    ProfileCategoryMemory,
			Name:        "Memory (inuse_objects)",
			Aggregation: ProfileAggregationAvg,
		},
		ProfileTypeGoHeapInuseSpace: {
			Category:    ProfileCategoryMemory,
			Name:        "Memory (inuse_space)",
			Aggregation: ProfileAggregationAvg,
			Featured:    true,
		},
		ProfileTypeGoGoroutines: {
			Name:        "Golang (goroutines)",
			Aggregation: ProfileAggregationAvg,
		},
		ProfileTypeGoBlockContentions: {
			Name:        "Golang (block_contentions)",
			Aggregation: ProfileAggregationSum,
		},
		ProfileTypeGoBlockDelay: {
			Name:        "Golang (block_delay)",
			Aggregation: ProfileAggregationSum,
		},
		ProfileTypeGoMutexContentions: {
			Name:        "Golang (mutex_contentions)",
			Aggregation: ProfileAggregationSum,
		},
		ProfileTypeGoMutexDelay: {
			Name:        "Golang (mutex_delay)",
			Aggregation: ProfileAggregationSum,
		},
	}
)

type FlameGraphNode struct {
	Name     string            `json:"name"`
	Total    int64             `json:"total"`
	Self     int64             `json:"self"`
	Comp     int64             `json:"comp"`
	Children []*FlameGraphNode `json:"children"`
	ColorBy  string            `json:"color_by"`
	Data     map[string]string `json:"data"`
}

func (n *FlameGraphNode) InsertStack(stack []string, value int64, comp *int64) {
	node := n
	l := len(stack) - 1
	for i := range stack {
		node.Total += value
		if comp != nil {
			node.Comp += *comp
		}
		name := stack[l-i]
		s := strings.IndexByte(name, ' ')
		if s > 0 {
			name = name[:s]
		}
		node = node.Insert(name)
	}
	node.Total += value
	node.Self += value
	if comp != nil {
		node.Comp += *comp
	}
}

func (n *FlameGraphNode) Insert(name string) *FlameGraphNode {
	i := sort.Search(len(n.Children), func(i int) bool {
		return strings.Compare(n.Children[i].Name, name) >= 0
	})
	if i > len(n.Children)-1 || n.Children[i].Name != name {
		child := &FlameGraphNode{Name: name}
		n.Children = append(n.Children, child)
		copy(n.Children[i+1:], n.Children[i:])
		n.Children[i] = child
	}
	return n.Children[i]
}

func (n *FlameGraphNode) Diff(comparison *FlameGraphNode) {
	n.diff(comparison)
	n.Comp = comparison.Total
	n.Total += n.Comp
}
func (n *FlameGraphNode) diff(comparison *FlameGraphNode) {
	byName := map[string]*FlameGraphNode{}
	if comparison != nil {
		for _, ch := range comparison.Children {
			byName[ch.Name] = ch
		}
	}
	seen := map[*FlameGraphNode]bool{}
	for _, ch := range n.Children {
		comp := byName[ch.Name]
		if byName[ch.Name] != nil {
			ch.Comp = comp.Total
			ch.Total += ch.Comp
			for k, v := range comp.Data {
				if ch.Data == nil {
					ch.Data = map[string]string{}
				}
				ch.Data[k] = v
			}
			seen[comp] = true
		}
		ch.diff(comp)
	}
	if comparison != nil {
		for _, ch := range comparison.Children {
			if !seen[ch] {
				ch.Comp = ch.Total
				n.Children = append(n.Children, ch)
			}
		}
	}
}
