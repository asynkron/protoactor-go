package lfqueue

import (
	"runtime"
	"sync"
	"testing"
)

func TestLfQueueConsistency(t *testing.T) {
	max := 100000000
	c := 10
	var wg sync.WaitGroup
	q := NewLockfreeQueue()
	go func() {
		i := 0
		for {
			r := q.Pop()
			if r == nil {
				runtime.Gosched()
			}
			i++
			if i == max {
				wg.Done()
				return
			}
		}
	}()

	for j := 0; j < c; j++ {
		cmax := max / c
		go func() {
			for i := 0; i < cmax; i++ {
				q.Push("abc")
			}
		}()
	}

	wg.Wait()
}
