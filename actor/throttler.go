package actor

import (
	"log/slog"
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

// NewThrottle
// This has no guarantees that the throttle opens exactly after the period, since it is reset asynchronously
// Throughput has been prioritized over exact re-opening
// throttledCallBack, This will be called with the number of events what was throttled after the period
func NewThrottle(maxEventsInPeriod int32, period time.Duration, throttledCallBack func(int32)) ShouldThrottle {
	currentEvents := int32(0)

	startTimer := func(duration time.Duration, back func(int32)) {
		go func() {
			// crete ticker to mimic sleep, we do not want to put the goroutine to sleep
			// as it will schedule it out of the P making a syscall, we just want it to
			// halt for the given period of time
			ticker := time.NewTicker(duration)
			defer ticker.Stop()
			<-ticker.C // wait for the ticker to tick once

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

func NewThrottleWithLogger(logger *slog.Logger, maxEventsInPeriod int32, period time.Duration, throttledCallBack func(*slog.Logger, int32)) ShouldThrottle {
	currentEvents := int32(0)

	startTimer := func(duration time.Duration, back func(*slog.Logger, int32)) {
		go func() {
			// crete ticker to mimic sleep, we do not want to put the goroutine to sleep
			// as it will schedule it out of the P making a syscall, we just want it to
			// halt for the given period of time
			ticker := time.NewTicker(duration)
			defer ticker.Stop()
			<-ticker.C // wait for the ticker to tick once

			timesCalled := atomic.SwapInt32(&currentEvents, 0)
			if timesCalled > maxEventsInPeriod {
				throttledCallBack(logger, timesCalled-maxEventsInPeriod)
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
