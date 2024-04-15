package model

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

type DependencyMapLink struct {
	SrcInstance string `json:"src_instance"`
	DstInstance string `json:"dst_instance"`

	Status Status `json:"status"`
}

type DependencyMap struct {
	Nodes []*DependencyMapNode `json:"nodes"`
	Links []*DependencyMapLink `json:"links"`
}

func (m *DependencyMap) GetOrCreateNode(node DependencyMapNode) *DependencyMapNode {
	if m == nil {
		return nil
	}
	for _, n := range m.Nodes {
		if n.Name == node.Name {
			return n
		}
	}
	m.Nodes = append(m.Nodes, &node)
	return &node
}

func (m *DependencyMap) UpdateLink(src DependencyMapInstance, sNode DependencyMapNode, dst DependencyMapInstance, dNode DependencyMapNode, linkStatus Status) {
	if m == nil {
		return
	}
	sn := m.GetOrCreateNode(sNode)
	sn.AddSrcInstance(src)
	dn := m.GetOrCreateNode(dNode)
	dn.AddDstInstance(dst)
	for _, l := range m.Links {
		if l.SrcInstance == src.Id && l.DstInstance == dst.Id {
			if l.Status < linkStatus {
				l.Status = linkStatus
			}
			return
		}
	}
	m.Links = append(m.Links, &DependencyMapLink{SrcInstance: src.Id, DstInstance: dst.Id, Status: linkStatus})
}
