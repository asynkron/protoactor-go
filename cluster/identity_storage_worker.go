package cluster

import (
	"log"

	"github.com/asynkron/protoactor-go/actor"
)

type IdentityStorageWorker struct {
	cluster *Cluster
	lookup  *IdentityStorageLookup
	storage StorageLookup
}

func newIdentityStorageWorker(storageLookup *IdentityStorageLookup) *IdentityStorageWorker {
	this := &IdentityStorageWorker{
		cluster: storageLookup.cluster,
		lookup:  storageLookup,
		storage: storageLookup.Storage,
	}
	return this
}

// Receive func
func (ids *IdentityStorageWorker) Receive(c actor.Context) {
	m := c.Message()
	getPid, ok := m.(GetPid)

	if !ok {
		return
	}

	if c.Sender() == nil {
		log.Println("No sender in GetPid request")
		return
	}

	existing, _ := ids.cluster.PidCache.Get(getPid.ClusterIdentity.Identity, getPid.ClusterIdentity.Kind)

	if existing != nil {
		log.Printf("Found %s in pidcache", m.(GetPid).ClusterIdentity.ToShortString())
		c.Respond(newPidResult(existing))
	}

	return
	// continue
}
