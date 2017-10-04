package members

//Rendezvous.go
//A revised FNV1A32 version of
//https://github.com/tysonmote/rendezvous/blob/master/rendezvous.go

import (
	"hash/fnv"
)

var hasher = fnv.New32a()

// Get returns the node with the highest score for the given key. If this Hash
// has no nodes, an empty string is returned.
func (m *MemberNodeSet) GetByRdv(key string) string {
	keyBytes := []byte(key)

	var maxScore uint32
	var maxNode *MemberNode
	var score uint32

	for _, node := range m.nodes {
		if node.Alive {
			score = hash(node.NameBytes, keyBytes)
			if score > maxScore {
				maxScore = score
				maxNode = node
			}
		}
	}

	return maxNode.Name
}

func hash(node, key []byte) uint32 {
	hasher.Reset()
	hasher.Write(key)
	hasher.Write(node)
	return hasher.Sum32()
}
