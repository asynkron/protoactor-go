package cluster

import (
	"log"

	"github.com/AsynkronIT/gam/actor"
	"github.com/hashicorp/memberlist"
)

func newClusterActor(list *memberlist.Memberlist) actor.ActorProducer {
	return func() actor.Actor {
		return &clusterActor{list: list}
	}
}

type clusterActor struct {
	list *memberlist.Memberlist
}

func (state *clusterActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		log.Println("Cluster actor started")
	default:
		log.Printf("Cluster got message %+v", msg)
	}
}
