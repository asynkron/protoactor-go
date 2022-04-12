package cluster

import (
	"github.com/asynkron/protoactor-go/actor"
	cmap "github.com/orcaman/concurrent-map"
)

type PidCacheValue struct {
	cache cmap.ConcurrentMap
}

func NewPidCache() *PidCacheValue {
	pidCache := &PidCacheValue{
		cache: cmap.New(),
	}

	return pidCache
}

func key(identity string, kind string) string {
	return identity + "." + kind
}

func (c *PidCacheValue) Get(identity string, kind string) (*actor.PID, bool) {
	k := key(identity, kind)
	v, ok := c.cache.Get(k)

	if !ok {
		return nil, false
	}

	return v.(*actor.PID), true
}

func (c *PidCacheValue) Set(identity string, kind string, pid *actor.PID) {
	k := key(identity, kind)
	c.cache.Set(k, pid)
}

func (c *PidCacheValue) RemoveByValue(identity string, kind string, pid *actor.PID) {
	k := key(identity, kind)

	c.cache.RemoveCb(k, func(key string, v interface{}, exists bool) bool {
		if !exists {
			return false
		}

		existing, _ := v.(*actor.PID)

		return existing.Equal(pid)
	})
}

func (c *PidCacheValue) Remove(identity string, kind string) {
	k := key(identity, kind)
	c.cache.Remove(k)
}

func (c *PidCacheValue) RemoveByMember(member *Member) {
	addr := member.Address()

	for item := range c.cache.IterBuffered() {
		pid, _ := item.Val.(*actor.PID)
		if pid.Address == addr {
			c.cache.Remove(item.Key)
		}
	}
}
