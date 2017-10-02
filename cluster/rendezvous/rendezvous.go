package rendezvous

//Rendezvous.go
//A revised FNV1A32 version of
//https://github.com/tysonmote/rendezvous/blob/master/rendezvous.go

import (
	"hash/fnv"
)

type MemberNodeSet map[string]*MemberNode

func (s MemberNodeSet) Add(key string, alive bool) { s[key] = NewMemberNode(key, alive) }
func (s MemberNodeSet) Remove(key string)          { delete(s, key) }

type MemberNode struct {
	Name      string
	NameBytes []byte
	Alive     bool
}

func NewMemberNode(name string, alive bool) *MemberNode {
	return &MemberNode{name, []byte(name), alive}
}

var hasher = fnv.New32a()

// Get returns the node with the highest score for the given key. If this Hash
// has no nodes, an empty string is returned.
func Get(set MemberNodeSet, key string) string {
	keyBytes := []byte(key)

	var maxScore uint32
	var maxNode *MemberNode
	var score uint32

	for _, node := range set {
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
