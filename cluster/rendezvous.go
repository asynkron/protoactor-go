package cluster

// Rendezvous.go
// A revised FNV1A32 version of
// https://github.com/tysonmote/rendezvous/blob/master/rendezvous.go

import (
	"hash"
	"hash/fnv"
	"strings"
	"sync"
)

type memberData struct {
	member    *Member
	hashBytes []byte
}
type Rendezvous struct {
	mutex      sync.RWMutex
	hasher     hash.Hash32
	hasherLock sync.Mutex
	members    []*memberData
}

func NewRendezvous() *Rendezvous {
	return &Rendezvous{
		hasher:  fnv.New32a(),
		members: make([]*memberData, 0),
	}
}

func (r *Rendezvous) GetByClusterIdentity(ci *ClusterIdentity) string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	identity := ci.Identity
	m := r.memberDataByKind(ci.Kind)

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
	parts := strings.SplitN(identity, "/", 2)

	return r.GetByClusterIdentity(&ClusterIdentity{
		Kind:     parts[0],
		Identity: parts[1],
	})
}

func (r *Rendezvous) memberDataByKind(kind string) []*memberData {
	m := make([]*memberData, 0)
	for _, md := range r.members {
		if md.member.HasKind(kind) {
			m = append(m, md)
		}
	}
	return m
}

func (r *Rendezvous) UpdateMembers(members Members) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	tmp := members.ToSet()
	r.members = make([]*memberData, 0)

	for _, m := range tmp.Members() {
		keyBytes := []byte(m.Address()) // TODO: should be utf8 to match .net
		r.members = append(r.members, &memberData{
			member:    m,
			hashBytes: keyBytes,
		})
	}
}

func (r *Rendezvous) hash(node, key []byte) uint32 {
	r.hasherLock.Lock()
	defer r.hasherLock.Unlock()

	r.hasher.Reset()
	r.hasher.Write(key)
	r.hasher.Write(node)
	return r.hasher.Sum32()
}
