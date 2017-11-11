package cluster

import (
	"sync"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
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
