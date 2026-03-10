package cluster

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGrainMetricsStore_Record(t *testing.T) {
	store := newGrainMetricsStore()

	before := time.Now()
	store.Record("kind/identity")
	store.Record("kind/identity")
	store.Record("kind/identity")
	after := time.Now()

	info := &GrainInfo{}
	store.applyTo("kind/identity", info)

	assert.Equal(t, int64(3), info.MessageCount)
	assert.False(t, info.LastMessageAt.IsZero())
	assert.True(t, info.LastMessageAt.After(before) || info.LastMessageAt.Equal(before))
	assert.True(t, info.LastMessageAt.Before(after) || info.LastMessageAt.Equal(after))
}

func TestGrainMetricsStore_Remove(t *testing.T) {
	store := newGrainMetricsStore()

	store.Record("kind/identity")
	store.Remove("kind/identity")

	info := &GrainInfo{}
	store.applyTo("kind/identity", info)

	assert.Equal(t, int64(0), info.MessageCount)
	assert.True(t, info.LastMessageAt.IsZero())
}

func TestGrainMetricsStore_ApplyToUnknownKey(t *testing.T) {
	store := newGrainMetricsStore()

	info := &GrainInfo{}
	store.applyTo("unknown/key", info)

	assert.Equal(t, int64(0), info.MessageCount)
	assert.True(t, info.LastMessageAt.IsZero())
}

func TestGrainMetricsStore_ConcurrentAccess(t *testing.T) {
	store := newGrainMetricsStore()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Record("kind/concurrent")
		}()
	}
	wg.Wait()

	info := &GrainInfo{}
	store.applyTo("kind/concurrent", info)
	assert.Equal(t, int64(100), info.MessageCount)
}
