package cluster

import (
	"log"
	"time"

	"github.com/AsynkronIT/gam/actor"
)

var (
	nameLookup = make(map[string]actor.Props)
)

//Register a known actor props by name
func Register(kind string, props actor.Props) {
	nameLookup[kind] = props
}

//Get a PID to a virtual actor
func Get(name string, kind string) (*actor.PID, error) {
	pid := cache.Get(name)
	if pid == nil {

		host := getNode(name)
		remote := partitionForHost(host)

		//request the pid of the "id" from the correct partition
		req := &ActorPidRequest{
			Name: name,
			Kind: kind,
		}

		//await the response
		res, err := remote.RequestFuture(req, 5*time.Second).Result()
		if err != nil {
			log.Printf("[CLUSTER] ActorPidRequest for '%v' timed out, failure %v", name, err)
			return nil, err
		}

		//unwrap the result
		typed, ok := res.(*ActorPidResponse)
		if !ok {
			log.Fatalf("[CLUSTER] ActorPidRequest for '%v' returned incorrect response, expected ActorPidResponse", name)
		}
		pid = typed.Pid
		cache.Add(name, pid)
		return pid, nil
	}
	return pid, nil
}
