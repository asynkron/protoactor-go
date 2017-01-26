package cluster

import (
	"math/rand"
	"time"

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

	req := &pidCacheRequest{
		kind: kind,
		name: name,
	}

	res, err := pidCacheActorPid.RequestFuture(req, 5*time.Second).Result()
	if err != nil {
		plog.Error("ActorPidRequest timed out", log.String("name", name), log.Error(err))
		return nil, err
	}
	typed, ok := res.(*remote.ActorPidResponse)
	if !ok {
		plog.Error("ActorPidRequest returned incorrect response", log.String("name", name))
	}
	return typed.Pid, nil
}
