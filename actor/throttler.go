package actor

import (
	"sync/atomic"
	"time"
)

type ShouldThrottle func() Valve

type Valve int32

const (
	Open Valve = iota
	Closing
	Closed
)

func NewThrottle(maxEventsInPeriod int32, period time.Duration, throttledCallBack func(int32)) ShouldThrottle {

	var currentEvents = int32(0)

	startTimer := func(duration time.Duration, back func(int32)) {
		go func() {
			time.Sleep(duration)
			timesCalled := atomic.SwapInt32(&currentEvents, 0)
			if timesCalled > maxEventsInPeriod {
				throttledCallBack(timesCalled - maxEventsInPeriod)
			}
		}()
	}

	return func() Valve {

		tries := atomic.AddInt32(&currentEvents, 1)
		if tries == 1 {
			startTimer(period, throttledCallBack)
		}

		if tries == maxEventsInPeriod {
			return Closing
		} else if tries > maxEventsInPeriod {
			return Closed
		} else {
			return Open
		}
	}
}
