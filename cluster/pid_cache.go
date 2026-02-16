package cluster

import (
	"time"

	"github.com/asynkron/protoactor-go/actor"
	cmap "github.com/orcaman/concurrent-map"
)

// pidCacheEntry wraps a PID with a creation timestamp for TTL-based expiration.
type pidCacheEntry struct {
	pid       *actor.PID
	createdAt time.Time
}

// PidCacheValue stores actor PIDs for quick lookups.
// When ttl is zero, entries never expire (backward-compatible default).
type PidCacheValue struct {
	cache cmap.ConcurrentMap
	ttl   time.Duration
}

// NewPidCache constructs a new PID cache with no expiration.
func NewPidCache() *PidCacheValue {
	return &PidCacheValue{
		cache: cmap.New(),
	}
}

// NewPidCacheWithTTL constructs a new PID cache where entries expire after
// the given duration. A zero TTL means entries never expire.
func NewPidCacheWithTTL(ttl time.Duration) *PidCacheValue {
	return &PidCacheValue{
		cache: cmap.New(),
		ttl:   ttl,
	}
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

	entry, isEntry := v.(*pidCacheEntry)
	if isEntry {
		if c.ttl > 0 && time.Since(entry.createdAt) > c.ttl {
			c.cache.Remove(k)
			return nil, false
		}

		return entry.pid, true
	}

	// Backward compatibility: support raw *actor.PID values (should not occur
	// in normal use, but keeps the code defensive).
	return v.(*actor.PID), true
}

func (c *PidCacheValue) Set(identity string, kind string, pid *actor.PID) {
	k := key(identity, kind)
	c.cache.Set(k, &pidCacheEntry{
		pid:       pid,
		createdAt: time.Now(),
	})
}

func (c *PidCacheValue) RemoveByValue(identity string, kind string, pid *actor.PID) {
	k := key(identity, kind)

	c.cache.RemoveCb(k, func(_ string, v any, exists bool) bool {
		if !exists {
			return false
		}

		if entry, ok := v.(*pidCacheEntry); ok {
			return entry.pid.Equal(pid)
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
		var pidAddr string
		if entry, ok := item.Val.(*pidCacheEntry); ok {
			pidAddr = entry.pid.Address
		} else if pid, ok := item.Val.(*actor.PID); ok {
			pidAddr = pid.Address
		}

		if pidAddr == addr {
			c.cache.Remove(item.Key)
		}
	}
}
