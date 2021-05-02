package cluster

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	cmap "github.com/orcaman/concurrent-map"
)

type pidCacheValue struct {
	cache cmap.ConcurrentMap
}

func NewPidCache() *pidCacheValue {
	pidCache := &pidCacheValue{
		cache: cmap.New(),
	}

	return pidCache
}

func key(identity string, kind string) string {
	return identity + "." + kind
}

func (c *pidCacheValue) Get(identity string, kind string) (*actor.PID, bool) {
	k := key(identity, kind)
	v, ok := c.cache.Get(k)
	if !ok {
		return nil, false
	}
	return v.(*actor.PID), true
}

func (c *pidCacheValue) Set(identity string, kind string, pid *actor.PID) {
	k := key(identity, kind)
	c.cache.Set(k, pid)
}

func (c *pidCacheValue) RemoveByValue(identity string, kind string, pid *actor.PID) {
	k := key(identity, kind)

	c.cache.RemoveCb(k, func(key string, v interface{}, exists bool) bool {
		if !exists {
			return false
		}

		existing := v.(*actor.PID)
		return existing.Equal(pid)
	})
}

func (c *pidCacheValue) Remove(identity string, kind string) {
	k := key(identity, kind)
	c.cache.Remove(k)
}

func (c *pidCacheValue) RemoveByMember(member *Member) {
	addr := member.Address()
	for item := range c.cache.IterBuffered() {
		pid := item.Val.(*actor.PID)
		if pid.Address == addr {
			c.cache.Remove(item.Key)
		}
	}
}
