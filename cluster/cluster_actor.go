package cluster

import (
	"log"
	"time"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/cluster/messages"
)

func clusterForHost(host string) *actor.PID {
	pid := actor.NewPID(host, "cluster")
	return pid
}

var clusterPid = actor.SpawnNamed(actor.FromProducer(newClusterActor()), "cluster")

func newClusterActor() actor.Producer {
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
		log.Printf("[CLUSTER] Node left %v", msg.node.host)
	case *messages.TakeOwnership:
		state.takeOwnership(msg)
	default:
		log.Printf("[CLUSTER] Cluster got unknown message %+v", msg)
	}
}

func (state *clusterActor) actorPidRequest(msg *messages.ActorPidRequest, context actor.Context) {

	pid := state.partition[msg.Name]
	if pid == nil {
		//get a random node
		random := getRandomActivator()

		//send request
		log.Printf("[CLUSTER] Telling %v to create %v", random, msg.Name)
		resp := random.RequestFuture(msg, 5*time.Second)
		tmp, err := resp.Result()
		if err != nil {
			log.Fatalf("Actor PID Request result failed %v", err)
		}
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
	log.Printf("[CLUSTER] Node joined %v", msg.node.host)
	if list.LocalNode() == nil {
		return
	}

	selfName := list.LocalNode().Name
	for actorID := range state.partition {
		host := getNode(actorID)
		if host != selfName {
			state.transferOwnership(actorID, host)
		}
	}
}

func (state *clusterActor) transferOwnership(actorID string, host string) {
	log.Printf("[CLUSTER] Giving ownership of %v to Node %v", actorID, host)
	pid := state.partition[actorID]
	owner := clusterForHost(host)
	owner.Tell(&messages.TakeOwnership{
		Pid:  pid,
		Name: actorID,
	})
	//we can safely delete this entry as the consisntent hash no longer points to us
	delete(state.partition, actorID)
}

func (state *clusterActor) takeOwnership(msg *messages.TakeOwnership) {
	log.Printf("[CLUSTER] Took ownerhip of %v", msg.Pid)
	state.partition[msg.Name] = msg.Pid
}
