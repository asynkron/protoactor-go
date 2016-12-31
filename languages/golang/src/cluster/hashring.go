package cluster

import (
	"hash/fnv"
	"math"
)

const (
	hashSize = uint32(math.MaxUint32)
)

func newClusterNode(host string) *clusterNode {
	return &clusterNode{
		host:  host,
		value: hash(host),
	}
}

type clusterNode struct {
	host  string
	value uint32
}

func (n *clusterNode) delta(v uint32) uint32 {
	d := delta(v, n.value)
	return d
}

func clusterNodes() []*clusterNode {
	m := list.Members()
	res := make([]*clusterNode, len(m))
	for i, n := range m {
		res[i] = newClusterNode(n.Name)
	}
	return res
}

func getNode(key string) string {
	v := hash(key)
	nodes := clusterNodes()
	bestV := hashSize
	bestI := 0

	//walk all members and find the node with the closest distance to the id hash
	for i, n := range nodes {
		if b := n.delta(v); b < bestV {
			bestV = b
			bestI = i
		}
	}

	node := nodes[bestI]
	return node.host
}

func delta(l uint32, r uint32) uint32 {
	if l > r {
		return l - r
	}
	return r - l
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
