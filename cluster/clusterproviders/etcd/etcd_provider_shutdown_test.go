package etcd

import (
	"sync"
	"testing"

	"github.com/asynkron/protoactor-go/cluster"
	"github.com/stretchr/testify/assert"
)

func TestProvider_ShutdownFlagIsAtomic(t *testing.T) {
	p := &Provider{
		members:         map[string]*Node{},
		cancelWatchCh:   make(chan bool),
		roleChangedChan: make(chan cluster.RoleType, 1),
	}

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
