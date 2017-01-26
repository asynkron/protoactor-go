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

type pidCacheResponse struct {
	pid       *actor.PID
	name      string
	kind      string
	respondTo *actor.PID
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

		address := getNode(msg.name, msg.kind)
		remotePID := partitionForKind(address, msg.kind)

		//we are about to go out of actor concurrency constraint
		//bind any Context information to vars
		//Do not do this at home..
		sender := ctx.Sender()
		self := ctx.Self()

		//re-package the request as a remote.ActorPidRequest
		req := &remote.ActorPidRequest{
			Kind: msg.kind,
			Name: msg.name,
		}
		//ask the DHT partition for this name to give us a PID
		f := remotePID.RequestFuture(req, 5*time.Second)
		ctx.AwaitFuture(f, func(r interface{}, err error) {
			if err != nil {
				return
			}
			typed, ok := r.(*remote.ActorPidResponse)
			if !ok {
				return
			}
			//repackage the ActorPidResonse as a pidCacheResponse + contextual information
			response := &pidCacheResponse{
				kind:      msg.kind,
				name:      msg.name,
				pid:       typed.Pid,
				respondTo: sender,
			}
			self.Tell(response)
		})

	case *pidCacheResponse:
		//add the pid to the cache using the name we requested
		a.Cache[msg.name] = msg.pid
		//make a lookup from pid to name
		a.ReverseCache[msg.pid.String()] = msg.name
		//watch the pid so we know if the node or pid dies
		ctx.Watch(msg.pid)
		//tell the original requester that we have a response
		msg.respondTo.Tell(&remote.ActorPidResponse{
			Pid: msg.pid,
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
