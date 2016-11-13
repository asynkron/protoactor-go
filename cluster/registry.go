package cluster

import (
	"hash/fnv"
	"log"
	"math/rand"
	"sort"
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

type byName []*memberlist.Node

func (s byName) Len() int {
	return len(s)
}
func (s byName) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

func getRandom() *actor.PID {
	r := rand.Int()
	members := list.Members()
	sort.Sort(byName(members))
	i := r % len(members)
	member := members[i]
	host := member.Name
	remote := actor.NewPID(host, "activator")
	return remote
}
func Get(id string, kind string) *actor.PID {
	h := int(hash(id))
	members := list.Members()
	sort.Sort(byName(members))
	i := h % len(members)
	member := members[i]
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
