package cluster

import (
	"hash/fnv"
	"log"

	"github.com/AsynkronIT/gam/actor"
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

func Get(id string, kind string) *actor.PID {
	h := int(hash(id))
	members := list.Members()
	i := h % len(members)
	member := members[i]
	host := member.Name
	remote := actor.NewPID(host, "cluster")
	log.Printf("Get Virtual %v %+v", id, remote)
	return remote
}
