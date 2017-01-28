package lfqueue

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestLfQueueConsistency(t *testing.T) {
	max := 1000000
	c := 10
	var wg sync.WaitGroup
	wg.Add(1)
	q := NewLockfreeQueue()
	go func() {
		i := 0
		for {
			r := q.Pop()
			if r == nil {
				runtime.Gosched()

				continue
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
				q.Push(fmt.Sprintf("%v %v", j, i))
			}
		}()
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)
	//queue should be empty
	for i := 0; i < 100; i++ {
		r := q.Pop()
		if r != nil {
			log.Printf("unexpected result %+v", r)
			t.FailNow()
		}
	}
}
