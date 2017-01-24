package cluster

import (
	"math/rand"
	"time"

	"os"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
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
		remotePID := partitionForKind(address, kind)

		//request the pid of the "id" from the correct partition
		req := &remote.ActorPidRequest{
			Name: name,
			Kind: kind,
		}

		//await the response
		res, err := remotePID.RequestFuture(req, 5*time.Second).Result()
		if err != nil {
			plog.Error("ActorPidRequest timed out", log.String("name", name), log.Error(err))
			return nil, err
		}

		//unwrap the result
		typed, ok := res.(*remote.ActorPidResponse)
		if !ok {
			plog.Error("ActorPidRequest returned incorrect response", log.String("name", name))
			os.Exit(1)
		}
		pid = typed.Pid
		cache.Add(name, pid)
		return pid, nil
	}
	return pid, nil
}
