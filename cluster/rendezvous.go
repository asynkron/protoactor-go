package cluster

// Rendezvous.go
// A revised FNV1A32 version of
// https://github.com/tysonmote/rendezvous/blob/master/rendezvous.go

import (
	"hash"
	"hash/fnv"
)

type memberData struct {
	member    *Member
	hashBytes []byte
}
type Rendezvous struct {
	hasher  hash.Hash32
	members []*memberData
}

func NewRendezvous() *Rendezvous {
	return &Rendezvous{fnv.New32a(), make([]*memberData, 0)}
}

func (r *Rendezvous) GetByClusterIdentity(ci *ClusterIdentity) string {
	identity := ci.Identity
	m := r.members
	//TODO: filter on kind ci.Kind

	l := len(m)

	if l == 0 {
		return ""
	}

	if l == 1 {
		return m[0].member.Address()
	}

	keyBytes := []byte(identity)

	var maxScore uint32
	var maxMember *memberData
	var score uint32

	for _, node := range m {
		score = r.hash(node.hashBytes, keyBytes)
		if score > maxScore {
			maxScore = score
			maxMember = node
		}
	}

	if maxMember == nil {
		return ""
	}
	return maxMember.member.Address()
}
func (r *Rendezvous) GetByIdentity(identity string) string {
	m := r.members
	l := len(m)

	if l == 0 {
		return ""
	}

	if l == 1 {
		return m[0].member.Address()
	}

	keyBytes := []byte(identity)

	var maxScore uint32
	var maxMember *memberData
	var score uint32

	for _, node := range m {
		score = r.hash(node.hashBytes, keyBytes)
		if score > maxScore {
			maxScore = score
			maxMember = node
		}
	}

	if maxMember == nil {
		return ""
	}
	return maxMember.member.Address()
}

func (r *Rendezvous) UpdateMembers(members Members) {
	//TODO: lock?
	//TODO: is this needed?
	tmp := CopySortMembers(members)
	r.members = make([]*memberData, 0)

	for _, m := range tmp {
		keyBytes := []byte(m.Address()) //TODO: should be utf8 to match .net
		r.members = append(r.members, &memberData{
			member:    m,
			hashBytes: keyBytes,
		})
	}
}

func (r *Rendezvous) hash(node, key []byte) uint32 {
	r.hasher.Reset()
	r.hasher.Write(key)
	r.hasher.Write(node)
	return r.hasher.Sum32()
}
