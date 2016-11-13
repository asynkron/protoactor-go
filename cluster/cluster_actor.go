package cluster

import (
	"log"
	"time"

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

			x, resp := actor.RequestResponsePID()
			//get a random node
			random := getRandom()

			//send request
			random.Tell(&messages.ActorActivateRequest{
				Id:     msg.Id,
				Kind:   msg.Kind,
				Sender: x,
			})

			tmp, _ := resp.ResultOrTimeout(5 * time.Second)
			typed := tmp.(*messages.ActorActivateResponse)
			pid = typed.Pid
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
