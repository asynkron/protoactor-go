package cluster

import (
	"log"
	"math"
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

type byValue []*memberlist.Node

func (s byValue) Len() int {
	return len(s)
}
func (s byValue) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byValue) Less(i, j int) bool {
	return getNodeValue(s[i]) < getNodeValue(s[j])
}

func getRandom() *actor.PID {
	r := rand.Int()
	members := list.Members()
	i := r % len(members)
	member := members[i]
	return activatorForNode(member)
}

func findClosest(id string) *memberlist.Node {
	h := int(hash(id))
	v := uint32(h % hashSize)

	members := list.Members()
	bestV := uint32(math.MaxUint32)
	bestI := 0

	for i, n := range members {
		nodeV := getNodeValue(n)
		abs := delta(v, nodeV)
		if abs < bestV {
			bestV = nodeV
			bestI = i
		}
	}
	log.Printf("Matching node value %v with node %v", v, bestV)

	member := members[bestI]
	return member
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
func Get(id string, kind string) *actor.PID {
	remote := clusterForNode(findClosest(id))
	future, response := actor.RequestResponsePID()

	//request the pid of the "id" from the correct partition
	req := &messages.ActorPidRequest{
		Id:     id,
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
	log.Printf("Get Virtual %v %+v", id, pid)
	return pid
}
