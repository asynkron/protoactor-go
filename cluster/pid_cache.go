package cluster

import (
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remote"
	"github.com/AsynkronIT/protoactor-go/router"
)

var (
	pidCacheActorPid *actor.PID
)

func spawnPidCacheActor() {
	props := router.NewConsistentHashPool(128).WithProducer(newPidCacheActor())
	pidCacheActorPid, _ = actor.SpawnNamed(props, "PidCache")

}
func newPidCacheActor() actor.Producer {
	return func() actor.Actor {
		return &pidCachePartitionActor{}
	}
}

type pidCachePartitionActor struct {
	Cache        map[string]*actor.PID
	ReverseCache map[string]string
}

type pidCacheRequest struct {
	name string
	kind string
}

func (p *pidCacheRequest) Hash() string {
	return p.name
}

func (a *pidCachePartitionActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		a.Cache = make(map[string]*actor.PID)
		a.ReverseCache = make(map[string]string)

	case *pidCacheRequest:
		if pid, ok := a.Cache[msg.name]; ok {
			//name was in cache, exit early
			ctx.Respond(&remote.ActorPidResponse{Pid: pid})
			return
		}
		name := msg.name
		kind := msg.kind

		address := getNode(name, kind)
		remotePID := partitionForKind(address, kind)

		//re-package the request as a remote.ActorPidRequest
		req := &remote.ActorPidRequest{
			Kind: kind,
			Name: name,
		}
		//ask the DHT partition for this name to give us a PID
		f := remotePID.RequestFuture(req, 5*time.Second)
		ctx.AwaitFuture(f, func(r interface{}, err error) {
			if err != nil {
				return
			}
			response, ok := r.(*remote.ActorPidResponse)
			if !ok {
				return
			}

			a.Cache[name] = response.Pid
			//make a lookup from pid to name
			a.ReverseCache[response.Pid.String()] = name
			//watch the pid so we know if the node or pid dies
			ctx.Watch(response.Pid)
			//tell the original requester that we have a response
			ctx.Respond(response)
		})

	case *actor.Terminated:
		key := msg.Who.String()
		//get the virtual name from the pid
		name, ok := a.ReverseCache[key]
		if !ok {
			//we don't have it, just ignore
			return
		}
		//drop both lookups as this actor is now dead
		delete(a.Cache, name)
		delete(a.ReverseCache, key)
	}
}
