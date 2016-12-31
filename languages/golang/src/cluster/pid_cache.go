package cluster

import (
	"sync"

	"github.com/AsynkronIT/gam/languages/golang/src/actor"
)

type pidCache struct {
	lock  sync.RWMutex
	Cache map[string]*actor.PID
}

func (c *pidCache) Get(key string) *actor.PID {
	c.lock.RLock()
	pid := c.Cache[key]
	c.lock.RUnlock()
	return pid
}

func (c *pidCache) Add(key string, pid *actor.PID) {
	c.lock.Lock()
	c.Cache[key] = pid
	c.lock.Unlock()
}

var cache = &pidCache{
	Cache: make(map[string]*actor.PID),
}
