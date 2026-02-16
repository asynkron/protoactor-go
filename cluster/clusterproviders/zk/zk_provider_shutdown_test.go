package zk

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvider_ShutdownFlagIsAtomic(t *testing.T) {
	p := &Provider{}

	var wg sync.WaitGroup
	wg.Add(2)

	// Writer goroutine
	go func() {
		defer wg.Done()
		p.shutdown.Store(true)
	}()

	// Reader goroutine
	go func() {
		defer wg.Done()
		_ = p.shutdown.Load()
	}()

	wg.Wait()
	assert.True(t, p.shutdown.Load())
}
