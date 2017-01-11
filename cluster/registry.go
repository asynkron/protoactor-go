package cluster

import (
	"log"
	"math/rand"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remoting"
)

func getRandomActivator(kind string) string {
	r := rand.Int()
	members := getMembers(kind)
	i := r % len(members)
	member := members[i]
	return member
}

//Get a PID to a virtual actor
func Get(name string, kind string) (*actor.PID, error) {
	pid := cache.Get(name)
	if pid == nil {

		address := getNode(name, kind)
		remote := partitionForKind(address, kind)

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
