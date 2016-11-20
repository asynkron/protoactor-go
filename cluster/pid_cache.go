package cluster

import (
	"sync"

	"github.com/AsynkronIT/gam/actor"
)

type pidCache struct {
	sync.RWMutex
	Cache map[string]*actor.PID
}

func (c *pidCache) Get(key string) *actor.PID {
	c.RLock()
	pid := c.Cache[key]
	c.RUnlock()
	return pid
}

func (c *pidCache) Add(key string, pid *actor.PID) {
	c.Lock()
	c.Cache[key] = pid
	c.Unlock()
}

var cache = &pidCache{
	Cache: make(map[string]*actor.PID),
}
