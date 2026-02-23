package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// blockingStore blocks forever on Get/Set until its context is cancelled.
type blockingStore struct{}

func (s *blockingStore) Get(ctx context.Context, key string) (*Subscribers, error) {
	<-ctx.Done()
	return nil, fmt.Errorf("store blocked: %w", ctx.Err())
}

func (s *blockingStore) Set(ctx context.Context, key string, value *Subscribers) error {
	<-ctx.Done()
	return fmt.Errorf("store blocked: %w", ctx.Err())
}

func (s *blockingStore) Clear(ctx context.Context, key string) error {
	<-ctx.Done()
	return fmt.Errorf("store blocked: %w", ctx.Err())
}

func TestTopicActor_loadSubscriptions_timesOut(t *testing.T) {
	t.Parallel()
	store := &blockingStore{}
	logger := slog.Default()
	timeout := 50 * time.Millisecond
	ta := NewTopicActor(store, logger, timeout)

	start := time.Now()
	subs := ta.loadSubscriptions("test-topic", logger)
	elapsed := time.Since(start)

	// Should return empty subscribers, not hang forever
	assert.NotNil(t, subs)
	assert.Empty(t, subs.Subscribers)
	// Should complete within a reasonable multiple of the timeout
	assert.Less(t, elapsed, 500*time.Millisecond)
}

func TestTopicActor_saveSubscriptions_timesOut(t *testing.T) {
	t.Parallel()
	store := &blockingStore{}
	logger := slog.Default()
	timeout := 50 * time.Millisecond
	ta := NewTopicActor(store, logger, timeout)
	ta.topic = "test-topic"

	start := time.Now()
	ta.saveSubscriptionsInTopicActor(logger)
	elapsed := time.Since(start)

	// Should complete within a reasonable multiple of the timeout
	assert.Less(t, elapsed, 500*time.Millisecond)
}
