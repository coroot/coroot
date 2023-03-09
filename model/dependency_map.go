package model

type DependencyMap struct {
	Nodes []*DependencyMapNode `json:"nodes"`
	Links []*DependencyMapLink `json:"links"`
}

type DependencyMapInstance struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Obsolete bool   `json:"obsolete"`
}

type DependencyMapNode struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Region   string `json:"region"`
	AZ       string `json:"az"`

	SrcInstances []DependencyMapInstance `json:"src_instances"`
	DstInstances []DependencyMapInstance `json:"dst_instances"`
}

type DependencyMapLink struct {
	SrcInstance string `json:"src_instance"`
	DstInstance string `json:"dst_instance"`

	Status Status `json:"status"`
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

func (m *DependencyMap) UpdateLink(src DependencyMapInstance, sNode DependencyMapNode, dst DependencyMapInstance, dNode DependencyMapNode, linkStatus Status) {
	sn := m.GetOrCreateNode(sNode)
	sn.AddSrcInstance(src)
	dn := m.GetOrCreateNode(dNode)
	dn.AddDstInstance(dst)
	m.Links = append(m.Links, &DependencyMapLink{SrcInstance: src.Id, DstInstance: dst.Id, Status: linkStatus})
}
