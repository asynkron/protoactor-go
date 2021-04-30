package identity

import (
	"log"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
)

type IdentityStorageWorker struct {
	cluster *cluster.Cluster
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
	_, ok := m.(GetPid)

	if !ok {
		return
	}

	if c.Sender() == nil {
		log.Println("No sender in GetPid request")
		return
	}

	cid := m.(GetPid).ClusterIdentity.Identity + "." + m.(GetPid).ClusterIdentity.Kind

	existing, _ := ids.cluster.PidCache.GetCache(cid)

	if existing != nil {
		log.Printf("Found %s in pidcache", m.(GetPid).ClusterIdentity.ToShortString())
		c.Respond(newPidResult(existing))
	}

	return
	// continue
}
