package cluster

import (
	"hash/fnv"
	"log"
	"math/rand"
	"time"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/cluster/messages"
	"github.com/hashicorp/memberlist"
)

var nameLookup = make(map[string]actor.Props)

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

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
	host := member.Name
	remote := actor.NewPID(host, "activator")
	return remote
}
func Get(id string, kind string) *actor.PID {
	h := int(hash(id))
	v := uint32(h % 32)

	members := list.Members()
	//	sort.Sort(byValue(members))
	bestV := uint32(9999)
	bestI := 0

	log.Printf("Member count %v", len(members))
	log.Printf("Current hash %v", v)
	for i, n := range members {
		nodeV := getNodeValue(n)
		log.Printf("Node %v value %v", n.Name, nodeV)

		abs := nodeV - v
		if v > nodeV {
			abs = v - nodeV
		}
		log.Printf("abs %v %v %v", abs, nodeV, v)
		if abs < bestV {
			bestV = nodeV
			bestI = i
		} else {
			log.Printf("not smaller")
		}
	}
	log.Printf("Matching node value %v with node %v", v, bestV)

	member := members[bestI]

	host := member.Name
	remote := actor.NewPID(host, "cluster")
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
