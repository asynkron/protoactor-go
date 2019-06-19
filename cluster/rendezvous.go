package cluster

// Rendezvous.go
// A revised FNV1A32 version of
// https://github.com/tysonmote/rendezvous/blob/master/rendezvous.go

import (
	"hash"
	"hash/fnv"
)

type Rendezvous struct {
	hasher       hash.Hash32
	m            MemberStrategy
	memberHashes [][]byte
}

func NewRendezvous(memberStrategy MemberStrategy) *Rendezvous {
	return &Rendezvous{fnv.New32a(), memberStrategy, make([][]byte, 0)}
}

// Get returns the node with the highest score for the given key. If this Hash
// has no nodes, an empty string is returned.
func (r *Rendezvous) GetByRdv(key string) string {
	members := r.m.GetAllMembers()
	l := len(members)

	if l == 0 {
		return ""
	}

	if l == 1 {
		return members[0].Address()
	}

	keyBytes := []byte(key)

	var maxScore uint32
	var maxMember *MemberStatus
	var score uint32

	for i, node := range members {
		if node.Alive {
			score = r.hash(r.memberHashes[i], keyBytes)
			if score > maxScore {
				maxScore = score
				maxMember = node
			}
		}
	}

	if maxMember != nil {
		return maxMember.Address()
	}
	return ""
}

func (r *Rendezvous) UpdateRdv() {
	r.memberHashes = make([][]byte, 0)
	for _, m := range r.m.GetAllMembers() {
		r.memberHashes = append(r.memberHashes, []byte(m.Address()))
	}
}

func (r *Rendezvous) hash(node, key []byte) uint32 {
	r.hasher.Reset()
	r.hasher.Write(key)
	r.hasher.Write(node)
	return r.hasher.Sum32()
}
