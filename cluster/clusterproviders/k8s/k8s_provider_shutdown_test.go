package k8s

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestProvider_ShutdownFlagIsAtomic(t *testing.T) {
	p := &Provider{
		clusterPods: make(map[types.UID]*v1.Pod),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		p.shutdown.Store(true)
	}()

	go func() {
		defer wg.Done()
		_ = p.shutdown.Load()
	}()

	wg.Wait()
	assert.True(t, p.shutdown.Load())
}
