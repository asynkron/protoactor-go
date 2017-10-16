package cluster

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/AsynkronIT/protoactor-go/log"
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
	Cache        map[string]*actor.PID
	ReverseCache map[string]string
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

type removePidCacheRequest struct {
	name string
}

func (p *removePidCacheRequest) Hash() string {
	return p.name
}

type pidCacheResponse struct {
	pid    *actor.PID
	status remote.ResponseStatusCode
}

func (a *pidCachePartitionActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		a.Cache = make(map[string]*actor.PID)
		a.ReverseCache = make(map[string]string)

	case *pidCacheRequest:
		if pid, ok := a.Cache[msg.name]; ok {
			//name was in cache, exit early
			ctx.Respond(&pidCacheResponse{pid: pid})
			return
		}
		name := msg.name
		kind := msg.kind

		address := getPartitionMember(name, kind)
		if address == "" {
			//No available member found
			ctx.Respond(&pidCacheResponse{status: remote.ResponseStatusCodeUNAVAILABLE})
			return
		}

		remotePartition := partitionForKind(address, kind)

		//re-package the request as a remote.ActorPidRequest
		req := &remote.ActorPidRequest{
			Kind: kind,
			Name: name,
		}
		//ask the DHT partition for this name to give us a PID
		f := remotePartition.RequestFuture(req, cfg.TimeoutTime)
		ctx.AwaitFuture(f, func(r interface{}, err error) {
			if err == actor.ErrTimeout {
				plog.Error("PidCache Pid request timeout")
				ctx.Respond(&pidCacheResponse{status: remote.ResponseStatusCodeTIMEOUT})
				return
			} else if err != nil {
				plog.Error("PidCache Pid request error", log.Error(err))
				ctx.Respond(&pidCacheResponse{status: remote.ResponseStatusCodeERROR})
				return
			}

			response, ok := r.(*remote.ActorPidResponse)
			if !ok {
				ctx.Respond(&pidCacheResponse{status: remote.ResponseStatusCodeERROR})
				return
			}

			statusCode := remote.ResponseStatusCode(response.StatusCode)
			switch statusCode {
			case remote.ResponseStatusCodeOK:
				key := response.Pid.String()

				a.Cache[name] = response.Pid
				//make a lookup from pid to name
				a.ReverseCache[key] = name

				//watch the pid so we know if the node or pid dies
				ctx.Watch(response.Pid)
				//tell the original requester that we have a response
				ctx.Respond(&pidCacheResponse{response.Pid, statusCode})
			default:
				//forward to requester
				ctx.Respond(&pidCacheResponse{response.Pid, statusCode})
			}
		})

	case *MemberLeftEvent:
		address := msg.Name()
		a.removeCacheByMemberAddress(address)
	case *MemberRejoinedEvent:
		address := msg.Name()
		a.removeCacheByMemberAddress(address)
	case *actor.Terminated:
		a.removeCacheByPid(msg.Who)
	case *removePidCacheRequest:
		a.removeCacheByName(msg.name)
	}
}

func (a *pidCachePartitionActor) removeCacheByPid(pid *actor.PID) {
	key := pid.String()
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

func (a *pidCachePartitionActor) removeCacheByName(name string) {
	if pid, ok := a.Cache[name]; ok {
		key := pid.String()
		delete(a.Cache, name)
		delete(a.ReverseCache, key)
	}
}

func (a *pidCachePartitionActor) removeCacheByMemberAddress(address string) {
	for name, pid := range a.Cache {
		if pid.Address == address {
			delete(a.Cache, name)
			delete(a.ReverseCache, pid.String())
		}
	}
}
