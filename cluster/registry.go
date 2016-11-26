package cluster

import (
	"log"
	"time"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/cluster/messages"
)

var (
	nameLookup = make(map[string]actor.Props)
)

//Register a known actor props by name
func Register(kind string, props actor.Props) {
	nameLookup[kind] = props
}

//Get a PID to a virtual actor
func Get(name string, kind string) *actor.PID {
	pid := cache.Get(name)
	if pid == nil {

		host := getNode(name)
		remote := clusterForHost(host)

		//request the pid of the "id" from the correct partition
		req := &messages.ActorPidRequest{
			Name: name,
			Kind: kind,
		}
		response := remote.AskFuture(req, 5*time.Second)

		//await the response
		res, err := response.Result()
		if err != nil {
			log.Fatalf("[CLUSTER DEBUG] response result failed %v", err)
		}

		//unwrap the result
		typed := res.(*messages.ActorPidResponse)
		pid = typed.Pid
		cache.Add(name, pid)
		return pid
	}
	return pid
}
