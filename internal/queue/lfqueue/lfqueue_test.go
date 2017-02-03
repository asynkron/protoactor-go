package lfqueue

import (
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestLfQueueConsistency(t *testing.T) {
	max := 1000000
	c := 100
	var wg sync.WaitGroup
	wg.Add(1)
	q := NewLockfreeQueue()
	go func() {
		i := 0
		seen := make(map[string]string)
		for {
			r := q.Pop()
			if r == nil {
				runtime.Gosched()

				continue
			}
			i++
			s := r.(string)
			_, present := seen[s]
			if present {
				log.Printf("duplicate item %v", s)
				t.FailNow()
			}
			seen[s] = s

			if i == max {
				wg.Done()
				return
			}
		}
	}()

	for j := 0; j < c; j++ {
		jj := j
		cmax := max / c
		go func() {
			for i := 0; i < cmax; i++ {
				if rand.Intn(10) == 0 {
					time.Sleep(time.Duration(rand.Intn(1000)))
				}
				q.Push(fmt.Sprintf("%v %v", jj, i))
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
