package cluster

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster/rendezvous"
	"github.com/AsynkronIT/protoactor-go/log"
)

func getNode(key, kind string) string {
	members := getMembers(kind)
	if members == nil {
		plog.Error("getNode: failed to get member", log.String("kind", kind))
		return actor.ProcessRegistry.Address
	}

	rdv := rendezvous.New(members...)
	return rdv.Get(key)
}
