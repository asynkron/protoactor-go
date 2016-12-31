package cluster

import (
	"log"
	"math/rand"
	"time"

	"github.com/AsynkronIT/protoactor/languages/golang/src/actor"
	"github.com/AsynkronIT/protoactor/languages/golang/src/remoting"
)

func getRandomActivator() string {
	r := rand.Int()
	members := list.Members()
	i := r % len(members)
	member := members[i]
	return member.Name
}

//Get a PID to a virtual actor
func Get(name string, kind string) (*actor.PID, error) {
	pid := cache.Get(name)
	if pid == nil {

		host := getNode(name)
		remote := partitionForHost(host)

		//request the pid of the "id" from the correct partition
		req := &remoting.ActorPidRequest{
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
		typed, ok := res.(*remoting.ActorPidResponse)
		if !ok {
			log.Fatalf("[CLUSTER] ActorPidRequest for '%v' returned incorrect response, expected ActorPidResponse", name)
		}
		pid = typed.Pid
		cache.Add(name, pid)
		return pid, nil
	}
	return pid, nil
}
