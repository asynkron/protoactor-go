package cluster

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
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
		return nil, err
	}
	typed, ok := res.(*remote.ActorPidResponse)
	if !ok {
		return nil, fmt.Errorf("Hej hopp")
	}
	return typed.Pid, nil
}
