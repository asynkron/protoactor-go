package cluster

import (
	"github.com/AsynkronIT/protoactor-go/log"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/AsynkronIT/protoactor-go/remote"
)

var (
	pidCacheActorPid *actor.PID
	memberStatusSub  *eventstream.Subscription
)

func spawnPidCacheActor() {
	props := actor.FromProducer(newPidCacheActor())
	pidCacheActorPid, _ = actor.SpawnNamed(props, "PidCache")
}

func stopPidCacheActor() {
	pidCacheActorPid.GracefulStop()
}

func newPidCacheActor() actor.Producer {
	return func() actor.Actor {
		return &pidCachePartitionActor{}
	}
}

func subscribePidCacheMemberStatusEventStream() {
	memberStatusSub = eventstream.
		Subscribe(pidCacheActorPid.Tell).
		WithPredicate(func(m interface{}) bool {
			_, ok := m.(MemberStatusEvent)
			return ok
		})
}

func unsubPidCacheMemberStatusEventStream() {
	eventstream.Unsubscribe(memberStatusSub)
}

type pidCachePartitionActor struct {
	Cache                       map[string]*actor.PID
	ReverseCache                map[string]string
	ReverseCacheByMemberAddress map[string]keySet
}

type keySet map[string]bool

func (s keySet) add(val string)    { s[val] = true }
func (s keySet) remove(val string) { delete(s, val) }

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
		a.ReverseCacheByMemberAddress = make(map[string]keySet)

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

			key := response.Pid.String()

			a.Cache[name] = response.Pid
			//make a lookup from pid to name
			a.ReverseCache[key] = name
			//add to member address map
			if ks, ok := a.ReverseCacheByMemberAddress[response.Pid.Address]; ok {
				ks.add(key)
			} else {
				a.ReverseCacheByMemberAddress[response.Pid.Address] = keySet{key: true}
			}

			//watch the pid so we know if the node or pid dies
			ctx.Watch(response.Pid)
			//tell the original requester that we have a response
			ctx.Respond(response)
		})

	case *MemberLeftEvent:
		address := msg.Name()
		a.removeCacheByMemberAddress(address)
	case *MemberRejoinedEvent:
		address := msg.Name()
		a.removeCacheByMemberAddress(address)
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
		if ks, ok := a.ReverseCacheByMemberAddress[msg.Who.Address]; ok {
			ks.remove(key)
		}
	}
}

func (a *pidCachePartitionActor) removeCacheByMemberAddress(address string) {
	if ks, ok := a.ReverseCacheByMemberAddress[address]; ok {
		for k := range ks {
			if n, ok := a.ReverseCache[k]; ok {
				delete(a.Cache, n)
				delete(a.ReverseCache, k)
			}
		}
		delete(a.ReverseCacheByMemberAddress, address)
		plog.Error("PID caches removed from PidCache", log.Object("address", address))
	}
}
