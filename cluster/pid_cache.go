package cluster

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	cmap "github.com/orcaman/concurrent-map"
)

var pidCache *pidCacheValue

type pidCacheValue struct {
	cache        cmap.ConcurrentMap
	reverseCache cmap.ConcurrentMap

	watcher         *actor.PID
	memberStatusSub *eventstream.Subscription
}

func setupPidCache() {
	pidCache = &pidCacheValue{
		cache:        cmap.New(),
		reverseCache: cmap.New(),
	}

	props := actor.PropsFromProducer(newPidCacheWatcher()).WithGuardian(actor.RestartingSupervisorStrategy())
	pidCache.watcher, _ = rootContext.SpawnNamed(props, "PidCacheWatcher")

	pidCache.memberStatusSub = eventstream.Subscribe(pidCache.onMemberStatusEvent).
		WithPredicate(func(m interface{}) bool {
			_, ok := m.(MemberStatusEvent)
			return ok
		})
}

func stopPidCache() {
	rootContext.StopFuture(pidCache.watcher).Wait()
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
	v, ok := c.cache.Get(name)
	if !ok {
		return nil, false
	}
	return v.(*actor.PID), true
}

func (c *pidCacheValue) addCache(name string, pid *actor.PID) bool {
	if c.cache.SetIfAbsent(name, pid) {
		key := pid.String()
		c.reverseCache.Set(key, name)
		// watch the pid so we know if the node or pid dies
		rootContext.Send(c.watcher, &watchPidRequest{pid})
		return true
	}
	return false
}

func (c *pidCacheValue) removeCacheByPid(pid *actor.PID) {
	key := pid.String()
	if name, ok := c.reverseCache.Get(key); ok {
		c.cache.Remove(name.(string))
		c.reverseCache.Remove(key)
	}
}

func (c *pidCacheValue) removeCacheByName(name string) {
	if pid, ok := c.cache.Get(name); ok {
		key := pid.(*actor.PID).String()
		c.cache.Remove(name)
		c.reverseCache.Remove(key)
	}
}

func (c *pidCacheValue) removeCacheByMemberAddress(address string) {
	for item := range c.cache.IterBuffered() {
		name := item.Key
		pid := item.Val.(*actor.PID)
		if pid.Address == address {
			c.cache.Remove(name)
			c.reverseCache.Remove(pid.String())
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
