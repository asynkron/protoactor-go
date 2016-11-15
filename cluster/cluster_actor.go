package cluster

import (
	"log"
	"time"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/cluster/messages"
)

var clusterPid = actor.SpawnNamed(actor.FromProducer(newClusterActor()), "cluster")

func newClusterActor() actor.ActorProducer {
	return func() actor.Actor {
		return &clusterActor{
			partition: make(map[string]*actor.PID),
		}
	}
}

type clusterActor struct {
	partition map[string]*actor.PID
}

func (state *clusterActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		log.Println("[CLUSTER] Cluster actor started")
	case *messages.ActorPidRequest:
		state.actorPidRequest(msg)
	case *clusterStatusJoin:
		state.clusterStatusJoin(msg)
	case *clusterStatusLeave:
		log.Printf("[CLUSTER] Node left %v", msg.node.Name)
	case *messages.TakeOwnership:
		log.Printf("[CLUSTER] Took ownerhip of %v", msg.Pid)
		state.partition[msg.Name] = msg.Pid
	default:
		log.Printf("[CLUSTER] Cluster got unknown message %+v", msg)
	}
}

func (state *clusterActor) actorPidRequest(msg *messages.ActorPidRequest) {
	log.Printf("%+v", msg)
	pid := state.partition[msg.Name]
	if pid == nil {
		x, resp := actor.RequestResponsePID()
		//get a random node
		random := getRandom()

		//send request
		log.Printf("[CLUSTER] Telling %v to create %v", random, msg.Name)
		random.Tell(&messages.ActorActivateRequest{
			Name:   msg.Name,
			Kind:   msg.Kind,
			Sender: x,
		})

		tmp, _ := resp.ResultOrTimeout(5 * time.Second)
		typed := tmp.(*messages.ActorActivateResponse)
		pid = typed.Pid
		state.partition[msg.Name] = pid
	}
	response := &messages.ActorPidResponse{
		Pid: pid,
	}
	msg.Sender.Tell(response)
}

func (state *clusterActor) clusterStatusJoin(msg *clusterStatusJoin) {
	log.Printf("[CLUSTER]  Node joined %v", msg.node.Name)
	selfName := list.LocalNode().Name
	for key := range state.partition {
		c := findClosest(key)
		if c.Name != selfName {
			log.Printf("[CLUSTER] Node %v should take ownership of %v", c.Name, key)
			pid := state.partition[key]
			owner := clusterForNode(c)
			owner.Tell(&messages.TakeOwnership{
				Pid:  pid,
				Name: key,
			})
		}
	}
}
