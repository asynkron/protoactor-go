package actor

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestThrottler(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// create a throttler that triggers after 10 invocations within 1 second
	throttler := NewThrottle(10, 1*time.Second, func(i int32) {
		wg.Done()
	})

	throttler()
	v := throttler()

	assert.Equal(t, Open, v)

	for i := 0; i < 8; i++ {
		v = throttler()
	}

	// should be closing now when we have invoked 10 times
	assert.Equal(t, Closing, v)

	// invoke once more
	v = throttler()
	// should bee closed, 11 invokes
	assert.Equal(t, Closed, v)

	// wait for callback to be invoked
	wg.Wait()

	// valve should be open now that time has elapsed
	v = throttler()
	assert.Equal(t, Open, v)
}
