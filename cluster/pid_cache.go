package cluster

import (
	"sync"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
)

var (
	pidCacheWatcher *actor.PID
	memberStatusSub *eventstream.Subscription
	pc              *pidCache
)

type pidCache struct {
	lock         *sync.Mutex
	cache        map[string]*actor.PID
	reverseCache map[string]string
}

func setupPidCache() {
	pc = &pidCache{
		lock:         &sync.Mutex{},
		cache:        make(map[string]*actor.PID),
		reverseCache: make(map[string]string),
	}
	props := actor.FromProducer(newPidCacheWatcher())
	pidCacheWatcher, _ = actor.SpawnNamed(props, "PidCacheWatcher")
}

func stopPidCache() {
	pidCacheWatcher.GracefulStop()
	pc = nil
}

func newPidCacheWatcher() actor.Producer {
	return func() actor.Actor {
		return &pidCacheWatcherActor{}
	}
}

func subscribePidCacheMemberStatusEventStream() {
	memberStatusSub = eventstream.
		Subscribe(onMemberStatusEvent).
		WithPredicate(func(m interface{}) bool {
			_, ok := m.(MemberStatusEvent)
			return ok
		})
}

func unsubPidCacheMemberStatusEventStream() {
	eventstream.Unsubscribe(memberStatusSub)
}

func onMemberStatusEvent(evn interface{}) {
	switch msEvn := evn.(type) {
	case *MemberLeftEvent:
		address := msEvn.Name()
		pc.removeCacheByMemberAddress(address)
	case *MemberRejoinedEvent:
		address := msEvn.Name()
		pc.removeCacheByMemberAddress(address)
	}
}

func getPid(name string, kind string) (*actor.PID, remote.ResponseStatusCode) {
	//Check Cache
	if pid, ok := pc.getCache(name); ok {
		return pid, remote.ResponseStatusCodeOK
	}

	//Get Pid
	address := getPartitionMember(name, kind)
	if address == "" {
		//No available member found
		return nil, remote.ResponseStatusCodeUNAVAILABLE
	}

	remotePartition := partitionForKind(address, kind)

	//package the request as a remote.ActorPidRequest
	req := &remote.ActorPidRequest{
		Kind: kind,
		Name: name,
	}

	//ask the DHT partition for this name to give us a PID
	f := remotePartition.RequestFuture(req, cfg.TimeoutTime)
	err := f.Wait()
	if err == actor.ErrTimeout {
		plog.Error("PidCache Pid request timeout")
		return nil, remote.ResponseStatusCodeTIMEOUT
	} else if err != nil {
		plog.Error("PidCache Pid request error", log.Error(err))
		return nil, remote.ResponseStatusCodeERROR
	}

	r, _ := f.Result()
	response, ok := r.(*remote.ActorPidResponse)
	if !ok {
		return nil, remote.ResponseStatusCodeERROR
	}

	statusCode := remote.ResponseStatusCode(response.StatusCode)
	switch statusCode {
	case remote.ResponseStatusCodeOK:
		//save cache
		pc.addCache(name, response.Pid)
		//watch the pid so we know if the node or pid dies
		pidCacheWatcher.Tell(&watchPidRequest{response.Pid})
		//tell the original requester that we have a response
		return response.Pid, statusCode
	default:
		//forward to requester
		return response.Pid, statusCode
	}
}

func (c *pidCache) getCache(name string) (*actor.PID, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	pid, ok := pc.cache[name]
	return pid, ok
}

func (c *pidCache) addCache(name string, pid *actor.PID) {
	c.lock.Lock()
	defer c.lock.Unlock()

	key := pid.String()
	c.cache[name] = pid
	c.reverseCache[key] = name
}

func (c *pidCache) removeCacheByPid(pid *actor.PID) {
	c.lock.Lock()
	defer c.lock.Unlock()

	key := pid.String()
	//get the virtual name from the pid
	name, ok := c.reverseCache[key]
	if !ok {
		//we don't have it, just ignore
		return
	}
	//drop both lookups as this actor is now dead
	delete(c.cache, name)
	delete(c.reverseCache, key)
}

func (c *pidCache) removeCacheByName(name string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if pid, ok := c.cache[name]; ok {
		key := pid.String()
		delete(c.cache, name)
		delete(c.reverseCache, key)
	}
}

func (c *pidCache) removeCacheByMemberAddress(address string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	for name, pid := range c.cache {
		if pid.Address == address {
			delete(c.cache, name)
			delete(c.reverseCache, pid.String())
		}
	}
}

type watchPidRequest struct {
	pid *actor.PID
}

type pidCacheWatcherActor struct{}

func (a *pidCacheWatcherActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *watchPidRequest:
		ctx.Watch(msg.pid)
	case *actor.Terminated:
		pc.removeCacheByPid(msg.Who)
	}
}
