package cluster

import (
	"sync"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
)

var pidCache *pidCacheValue

type pidCacheValue struct {
	lock         *sync.Mutex
	cache        map[string]*actor.PID
	reverseCache map[string]string

	watcher         *actor.PID
	memberStatusSub *eventstream.Subscription
}

func setupPidCache() {
	pidCache = &pidCacheValue{
		lock:         &sync.Mutex{},
		cache:        make(map[string]*actor.PID),
		reverseCache: make(map[string]string),
	}

	props := actor.FromProducer(newPidCacheWatcher())
	pidCache.watcher, _ = actor.SpawnNamed(props, "PidCacheWatcher")

	pidCache.memberStatusSub = eventstream.Subscribe(pidCache.onMemberStatusEvent).
		WithPredicate(func(m interface{}) bool {
			_, ok := m.(MemberStatusEvent)
			return ok
		})
}

func stopPidCache() {
	pidCache.watcher.GracefulStop()
	eventstream.Unsubscribe(pidCache.memberStatusSub)
	pidCache = nil
}

func (c *pidCacheValue) onMemberStatusEvent(evn interface{}) {
	switch msEvn := evn.(type) {
	case *MemberLeftEvent:
		address := msEvn.Name()
		c.removeCacheByMemberAddress(address)
	case *MemberRejoinedEvent:
		address := msEvn.Name()
		c.removeCacheByMemberAddress(address)
	}
}

func (c *pidCacheValue) getCache(name string) (*actor.PID, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	pid, ok := pidCache.cache[name]
	return pid, ok
}

func (c *pidCacheValue) addCache(name string, pid *actor.PID) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, ok := c.cache[name]; ok {
		return false
	}

	key := pid.String()
	c.cache[name] = pid
	c.reverseCache[key] = name
	//watch the pid so we know if the node or pid dies
	c.watcher.Tell(&watchPidRequest{pid})
	return true
}

func (c *pidCacheValue) removeCacheByPid(pid *actor.PID) {
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

func (c *pidCacheValue) removeCacheByName(name string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if pid, ok := c.cache[name]; ok {
		key := pid.String()
		delete(c.cache, name)
		delete(c.reverseCache, key)
	}
}

func (c *pidCacheValue) removeCacheByMemberAddress(address string) {
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

func newPidCacheWatcher() actor.Producer {
	return func() actor.Actor {
		return &pidCacheWatcherActor{}
	}
}

func (a *pidCacheWatcherActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *watchPidRequest:
		ctx.Watch(msg.pid)
	case *actor.Terminated:
		pidCache.removeCacheByPid(msg.Who)
	}
}
