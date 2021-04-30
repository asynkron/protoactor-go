package cluster

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	cmap "github.com/orcaman/concurrent-map"
)

type pidCacheValue struct {
	cache        cmap.ConcurrentMap
	reverseCache cmap.ConcurrentMap
	actorSystem  *actor.ActorSystem
}

func setupPidCache(actorSystem *actor.ActorSystem) *pidCacheValue {
	pidCache := &pidCacheValue{
		cache:        cmap.New(),
		reverseCache: cmap.New(),
		actorSystem:  actorSystem,
	}

	return pidCache
}

func (c *pidCacheValue) GetCache(name string) (*actor.PID, bool) {
	v, ok := c.cache.Get(name)
	if !ok {
		return nil, false
	}
	return v.(*actor.PID), true
}

func (c *pidCacheValue) AddCache(name string, pid *actor.PID) bool {
	if c.cache.SetIfAbsent(name, pid) {
		key := pid.String()
		c.reverseCache.Set(key, name)
		return true
	}
	return false
}

func (c *pidCacheValue) RemoveCacheByPid(pid *actor.PID) {
	key := pid.String()
	if name, ok := c.reverseCache.Get(key); ok {
		c.cache.Remove(name.(string))
		c.reverseCache.Remove(key)
	}
}

func (c *pidCacheValue) RemoveCacheByName(name string) {
	if pid, ok := c.cache.Get(name); ok {
		key := pid.(*actor.PID).String()
		c.cache.Remove(name)
		c.reverseCache.Remove(key)
	}
}

func (c *pidCacheValue) RemoveCacheByMemberAddress(address string) {
	for item := range c.cache.IterBuffered() {
		name := item.Key
		pid := item.Val.(*actor.PID)
		if pid.Address == address {
			c.cache.Remove(name)
			c.reverseCache.Remove(pid.String())
		}
	}
}
