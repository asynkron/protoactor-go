package cluster

import (
	"log"
	"math/rand"
	"time"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/cluster/messages"
	"github.com/hashicorp/memberlist"
)

var nameLookup = make(map[string]actor.Props)

//Register a known actor props by name
func Register(kind string, props actor.Props) {
	nameLookup[kind] = props
}

func getRandom() *actor.PID {
	r := rand.Int()
	members := list.Members()
	i := r % len(members)
	member := members[i]
	return activatorForNode(member)
}

func findClosest(id string) *memberlist.Node {
	v := hash(id)

	members := members()
	bestV := hashSize
	bestI := 0

	//walk all members and find the node with the closest distance to the id hash
	for i, n := range members {
		if n.delta(v) < bestV {
			bestV = n.value
			bestI = i
		}
	}
	log.Printf("[CLUSTER] Matching node value %v with node %v", v, bestV)

	member := members[bestI]
	return member.Node
}

func clusterForNode(node *memberlist.Node) *actor.PID {
	host := node.Name
	pid := actor.NewPID(host, "cluster")
	return pid
}

func activatorForNode(node *memberlist.Node) *actor.PID {
	host := node.Name
	pid := actor.NewPID(host, "activator")
	return pid
}

//Get a PID to a virtual actor
func Get(name string, kind string) *actor.PID {
	remote := clusterForNode(findClosest(name))
	future, response := actor.RequestResponsePID()

	//request the pid of the "id" from the correct partition
	req := &messages.ActorPidRequest{
		Name:   name,
		Sender: future,
		Kind:   kind,
	}
	remote.Tell(req)

	//await the response
	res, err := response.ResultOrTimeout(5 * time.Second)
	if err != nil {
		log.Fatal(err)
	}

	//unwrap the result
	typed := res.(*messages.ActorPidResponse)
	pid := typed.Pid
	log.Printf("[CLUSTER] Get Virtual %v %+v", name, pid)
	return pid
}
