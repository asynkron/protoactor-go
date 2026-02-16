package mpsc

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestQueue_PushPop(t *testing.T) {
	q := New()

	q.Push(1)
	q.Push(2)
	assert.Equal(t, 1, q.Pop())
	assert.Equal(t, 2, q.Pop())
	assert.True(t, q.Empty())
}

func TestQueue_Empty(t *testing.T) {
	q := New()
	assert.True(t, q.Empty())
	q.Push(1)
	assert.False(t, q.Empty())
}

func TestQueue_PushPopOneProducer(_ *testing.T) {
	expCount := 100

	var wg sync.WaitGroup
	wg.Add(1)
	q := New()
	go func() {
		i := 0
		for {
			r := q.Pop()
			if r == nil {
				runtime.Gosched()
				continue
			}
			i++
			if i == expCount {
				wg.Done()
				return
			}
		}
	}()

	var val any = "foo"

	for i := 0; i < expCount; i++ {
		q.Push(val)
	}

	wg.Wait()
}

func TestMpscQueueConsistency(t *testing.T) {
	max := 1000000
	c := runtime.NumCPU() / 2
	if c < 1 {
		c = 1
	}
	cmax := max / c
	var wg sync.WaitGroup
	wg.Add(1)
	q := New()

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
			s, _ := r.(string)
			_, present := seen[s]
			if present {
				t.Errorf("item has already been seen %v", s)
				wg.Done()
				return
			}
			seen[s] = s
			if i == cmax*c {
				wg.Done()
				return
			}
		}
	}()

	for j := 0; j < c; j++ {
		jj := j
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
	// queue should be empty
	for i := 0; i < 100; i++ {
		r := q.Pop()
		if r != nil {
			t.Errorf("unexpected result %+v", r)
		}
	}
}

func benchmarkPushPop(count, c int) {
	var wg sync.WaitGroup
	wg.Add(1)
	q := New()
	go func() {
		i := 0
		for {
			r := q.Pop()
			if r == nil {
				runtime.Gosched()
				continue
			}
			i++
			if i == count {
				wg.Done()
				return
			}
		}
	}()

	var val any = "foo"

	for i := 0; i < c; i++ {
		go func(n int) {
			for n > 0 {
				q.Push(val)
				n--
			}
		}(count / c)
	}

	wg.Wait()
}

func benchmarkChannelPushPop(count, c int) {
	var wg sync.WaitGroup
	wg.Add(1)
	ch := make(chan any, 100)
	go func() {
		i := 0
		for {
			<-ch
			i++
			if i == count {
				wg.Done()
				return
			}
		}
	}()

	var val any = "foo"

	for i := 0; i < c; i++ {
		go func(n int) {
			for n > 0 {
				ch <- val
				n--
			}
		}(count / c)
	}
}

func BenchmarkPushPop(b *testing.B) {
	benchmarks := []struct {
		count       int
		concurrency int
	}{
		{
			count:       10000,
			concurrency: 1,
		},
		{
			count:       10000,
			concurrency: 2,
		},
		{
			count:       10000,
			concurrency: 4,
		},
		{
			count:       10000,
			concurrency: 8,
		},
	}
	for _, bm := range benchmarks {
		b.Run(fmt.Sprintf("%d_%d", bm.count, bm.concurrency), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				benchmarkPushPop(bm.count, bm.concurrency)
			}
		})
	}
}

func BenchmarkChannelPushPop(b *testing.B) {
	benchmarks := []struct {
		count       int
		concurrency int
	}{
		{
			count:       10000,
			concurrency: 1,
		},
		{
			count:       10000,
			concurrency: 2,
		},
		{
			count:       10000,
			concurrency: 4,
		},
		{
			count:       10000,
			concurrency: 8,
		},
	}
	for _, bm := range benchmarks {
		b.Run(fmt.Sprintf("%d_%d", bm.count, bm.concurrency), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				benchmarkChannelPushPop(bm.count, bm.concurrency)
			}
		})
	}
}
