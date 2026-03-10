package cluster

import (
	"sync"
	"sync/atomic"
	"time"
)

// grainMetricsEntry stores per-grain message statistics.
type grainMetricsEntry struct {
	lastMessageAt atomic.Int64 // unix nanoseconds
	messageCount  atomic.Int64
}

// grainMetricsStore is a concurrent map of identity key to metrics entry.
type grainMetricsStore struct {
	entries sync.Map // map[string]*grainMetricsEntry
}

func newGrainMetricsStore() *grainMetricsStore {
	return &grainMetricsStore{}
}

// Record updates the metrics for a grain identified by its key.
func (s *grainMetricsStore) Record(key string) {
	v, _ := s.entries.LoadOrStore(key, &grainMetricsEntry{})
	entry := v.(*grainMetricsEntry)
	entry.lastMessageAt.Store(time.Now().UnixNano())
	entry.messageCount.Add(1)
}

// Remove deletes the metrics entry for a grain.
func (s *grainMetricsStore) Remove(key string) {
	s.entries.Delete(key)
}

// applyTo merges the stored metrics into a GrainInfo.
func (s *grainMetricsStore) applyTo(key string, info *GrainInfo) {
	v, ok := s.entries.Load(key)
	if !ok {
		return
	}
	entry := v.(*grainMetricsEntry)
	if nanos := entry.lastMessageAt.Load(); nanos > 0 {
		info.LastMessageAt = time.Unix(0, nanos)
	}
	info.MessageCount = entry.messageCount.Load()
}
