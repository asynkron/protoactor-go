package cluster

import (
	"log"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/cluster/messages"
	"github.com/hashicorp/memberlist"
)

func newClusterActor(list *memberlist.Memberlist) actor.ActorProducer {
	return func() actor.Actor {
		return &clusterActor{
			list:      list,
			partition: make(map[string]*actor.PID),
		}
	}
}

type clusterActor struct {
	list      *memberlist.Memberlist
	partition map[string]*actor.PID
}

func (state *clusterActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		log.Println("Cluster actor started")
	case *messages.ActorPidRequest:
		pid := state.partition[msg.Id]
		if pid == nil {
			log.Printf("Cluster actor creating %v of type %v", msg.Id, msg.Kind)
			props := nameLookup[msg.Kind]
			pid = actor.SpawnNamed(props, msg.Id)
			state.partition[msg.Id] = pid
		}
		response := &messages.ActorPidResponse{
			Pid: pid,
		}
		msg.Sender.Tell(response)

	default:
		log.Printf("Cluster got unknown message %+v", msg)
	}
}
