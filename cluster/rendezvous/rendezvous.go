package rendezvous

//Rendezvous.go
//A FNV1A32 version of
//https://github.com/tysonmote/rendezvous/blob/master/rendezvous.go

import (
	"hash"
	"hash/fnv"
	"sort"
)

type nodeScore struct {
	node  []byte
	score uint32
}

type byScore []nodeScore

func (s byScore) Len() int           { return len(s) }
func (s byScore) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s byScore) Less(i, j int) bool { return s[i].score < s[j].score }

type Hash struct {
	nodes  []nodeScore
	hasher hash.Hash32
}

// New creates a new Hash with the given keys (optional).
func New(nodes ...string) *Hash {
	hash := &Hash{}
	hash.hasher = fnv.New32a()
	hash.Add(nodes...)
	return hash
}

// Add takes any number of nodes and adds them to this Hash.
func (h *Hash) Add(nodes ...string) {
	for _, node := range nodes {
		h.nodes = append(h.nodes, nodeScore{[]byte(node), 0})
	}
}

// Get returns the node with the highest score for the given key. If this Hash
// has no nodes, an empty string is returned.
func (h *Hash) Get(key string) string {
	keyBytes := []byte(key)

	var maxScore uint32
	var maxNode []byte
	var score uint32

	for _, node := range h.nodes {
		score = h.hash(node.node, keyBytes)
		if score > maxScore {
			maxScore = score
			maxNode = node.node
		}
	}

	return string(maxNode)
}

// GetN returns n nodes for the given key, ordered by descending score.
func (h *Hash) GetN(n int, key string) []string {
	if len(h.nodes) == 0 || n == 0 {
		return []string{}
	}

	if n > len(h.nodes) {
		n = len(h.nodes)
	}

	keyBytes := []byte(key)
	for i := 0; i < len(h.nodes); i++ {
		h.nodes[i].score = h.hash(h.nodes[i].node, keyBytes)
	}
	sort.Sort(sort.Reverse(byScore(h.nodes)))

	nodes := make([]string, n)
	for i := 0; i < n; i++ {
		nodes[i] = string(h.nodes[i].node)
	}
	return nodes
}

func (h *Hash) hash(node, key []byte) uint32 {
	h.hasher.Reset()
	h.hasher.Write(key)
	h.hasher.Write(node)
	return h.hasher.Sum32()
}
