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
	//log.Printf("%+v", context.Message())
	switch msg := context.Message().(type) {
	case *actor.Started:
		log.Println("[CLUSTER] Cluster actor started")
	case *messages.ActorPidRequest:
		state.actorPidRequest(msg, context)
	case *clusterStatusJoin:
		state.clusterStatusJoin(msg)
	case *clusterStatusLeave:
		log.Printf("[CLUSTER] Node left %v", msg.node.Name)
	case *messages.TakeOwnership:
		state.takeOwnership(msg)
	default:
		log.Printf("[CLUSTER] Cluster got unknown message %+v", msg)
	}
}
func (state *clusterActor) takeOwnership(msg *messages.TakeOwnership) {
	log.Printf("[CLUSTER] Took ownerhip of %v", msg.Pid)
	state.partition[msg.Name] = msg.Pid
}

func (state *clusterActor) actorPidRequest(msg *messages.ActorPidRequest, context actor.Context) {

	pid := state.partition[msg.Name]
	if pid == nil {
		//get a random node
		random := getRandomActivator()

		//send request
		log.Printf("[CLUSTER] Telling %v to create %v", random, msg.Name)
		resp, _ := random.Ask(msg)
		defer resp.Stop()

		tmp, _ := resp.ResultOrTimeout(5 * time.Second)
		typed := tmp.(*messages.ActorPidResponse)
		pid = typed.Pid
		state.partition[msg.Name] = pid
	}
	response := &messages.ActorPidResponse{
		Pid: pid,
	}
	context.Sender().Tell(response)
}

func (state *clusterActor) clusterStatusJoin(msg *clusterStatusJoin) {
	log.Printf("[CLUSTER] Node joined %v", msg.node.Name)
	if list.LocalNode() == nil {
		return
	}

	selfName := list.LocalNode().Name
	for key := range state.partition {
		c := findClosest(key)
		if c.Name != selfName {
			log.Printf("[CLUSTER] Giving ownership of %v to Node %v", key, c.Name)
			pid := state.partition[key]
			owner := clusterForNode(c)
			owner.Tell(&messages.TakeOwnership{
				Pid:  pid,
				Name: key,
			})
			//we can safely delete this entry as the consisntent hash no longer points to us
			delete(state.partition, key)
		}
	}
}
