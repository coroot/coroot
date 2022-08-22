package view

import (
	"github.com/coroot/coroot-focus/model"
)

type DependencyMap struct {
	Nodes []*DependencyMapNode
	Links []*DependencyMapLink
}

type DependencyMapInstance struct {
	Name     string
	Obsolete bool
}

type DependencyMapNode struct {
	Name         string
	Provider     string
	Region       string
	AZ           string
	SrcInstances []DependencyMapInstance
	DstInstances []DependencyMapInstance
}

func (m *DependencyMap) GetOrCreateNode(node DependencyMapNode) *DependencyMapNode {
	for _, n := range m.Nodes {
		if n.Name == node.Name {
			return n
		}
	}
	m.Nodes = append(m.Nodes, &node)
	return &node
}

func (n *DependencyMapNode) AddSrcInstance(i DependencyMapInstance) {
	for _, ii := range n.SrcInstances {
		if ii.Name == i.Name {
			return
		}
	}
	n.SrcInstances = append(n.SrcInstances, i)
}

func (n *DependencyMapNode) AddDstInstance(i DependencyMapInstance) {
	for _, ii := range n.DstInstances {
		if ii.Name == i.Name {
			return
		}
	}
	n.DstInstances = append(n.DstInstances, i)
}

func (m *DependencyMap) UpdateLink(src DependencyMapInstance, sNode DependencyMapNode, dst DependencyMapInstance, dNode DependencyMapNode, linkStatus model.Status) {
	sn := m.GetOrCreateNode(sNode)
	sn.AddSrcInstance(src)
	dn := m.GetOrCreateNode(dNode)
	dn.AddDstInstance(dst)
	m.Links = append(m.Links, &DependencyMapLink{SrcInstance: src.Name, DstInstance: dst.Name, Status: linkStatus})
}

type DependencyMapLink struct {
	SrcInstance string
	DstInstance string
	Status      model.Status
}
