package cluster

import (
	"github.com/cespare/xxhash"
	rdv "github.com/dgryski/go-rendezvous"
)

// RendezvousV2 ...
type RendezvousV2 struct {
	rdv *rdv.Rendezvous
}

// NewRendezvousV2 ...
func NewRendezvousV2(members []*Member) *RendezvousV2 {
	addrs := make([]string, len(members))
	for i, member := range members {
		addrs[i] = member.Address()
	}
	return &RendezvousV2{
		rdv: rdv.New(addrs, xxhash.Sum64String),
	}
}

// Get ...
func (r *RendezvousV2) Get(key string) string {
	return r.rdv.Lookup(key)
}
