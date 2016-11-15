package cluster

import "github.com/hashicorp/memberlist"

func getClusterNode(node *memberlist.Node) *clusterNode {
	return &clusterNode{
		Node:  node,
		value: hash(node.Name),
	}
}

type clusterNode struct {
	*memberlist.Node
	value uint32
}

func (n *clusterNode) delta(v uint32) uint32 {
	d := delta(v, n.value)
	return d
}

func members() []*clusterNode {
	m := list.Members()
	res := make([]*clusterNode, len(m))
	for i, n := range m {
		res[i] = getClusterNode(n)
	}
	return res
}
