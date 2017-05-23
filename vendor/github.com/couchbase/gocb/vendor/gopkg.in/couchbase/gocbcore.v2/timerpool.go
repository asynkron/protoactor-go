package gocbcore

import (
	"sync"
	"time"
)

var globalTimerPool sync.Pool

func AcquireTimer(d time.Duration) *time.Timer {
	tmrMaybe := globalTimerPool.Get()
	if tmrMaybe == nil {
		return time.NewTimer(d)
	}
	tmr := tmrMaybe.(*time.Timer)
	tmr.Reset(d)
	return tmr
}

func ReleaseTimer(t *time.Timer, wasRead bool) {
	stopped := t.Stop()
	if !wasRead && !stopped {
		<-t.C
	}
	globalTimerPool.Put(t)
}
