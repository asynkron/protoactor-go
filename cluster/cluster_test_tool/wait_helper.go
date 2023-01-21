package cluster_test_tool

import (
	"runtime/debug"
	"testing"
	"time"
)

const DefaultWaitTimeout = time.Second * 5

func WaitUntil(t testing.TB, cond func() bool, errorMsg string, timeout time.Duration) {
	after := time.After(timeout)

	for {
		select {
		case <-after:
			t.Error(errorMsg)
			debug.PrintStack()
		default:
			if cond() {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}
