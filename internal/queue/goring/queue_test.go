package goring

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPushPop(t *testing.T) {
	q := New(10)
	q.Push("hello")
	res, _ := q.Pop()
	assert.Equal(t, "hello", res)
	assert.True(t, q.Empty())
}

func TestPushPopRepeated(t *testing.T) {
	q := New(10)
	for i := 0; i < 100; i++ {
		q.Push("hello")
		res, _ := q.Pop()
		assert.Equal(t, "hello", res)
		assert.True(t, q.Empty())
	}
}

func TestPushPopMany(t *testing.T) {
	q := New(10)
	for i := 0; i < 10000; i++ {
		item := fmt.Sprintf("hello%v", i)
		q.Push(item)
		res, _ := q.Pop()
		assert.Equal(t, item, res)
	}
	assert.True(t, q.Empty())
}

func TestPushPopMany2(t *testing.T) {
	q := New(10)
	for i := 0; i < 10000; i++ {
		item := fmt.Sprintf("hello%v", i)
		q.Push(item)
	}
	for i := 0; i < 10000; i++ {
		item := fmt.Sprintf("hello%v", i)
		res, _ := q.Pop()
		assert.Equal(t, item, res)
	}
	assert.True(t, q.Empty())
}

//func TestLfQueueConsistency(t *testing.T) {
//	max := 1000000
//	c := 100
//	var wg sync.WaitGroup
//	wg.Add(1)
//	q := New(2)
//	go func() {
//		i := 0
//		seen := make(map[string]string)
//		for {
//			r, ok := q.Pop()
//			if !ok {
//				runtime.Gosched()
//
//				continue
//			}
//			i++
//			if r == nil {
//				log.Printf("%#v, %#v", q, q.content)
//				panic("consistency failure")
//			}
//			s := r.(string)
//			_, present := seen[s]
//			if present {
//				log.Printf("item have already been seen %v", s)
//				t.FailNow()
//			}
//			seen[s] = s
//
//			if i == max {
//				wg.Done()
//				return
//			}
//		}
//	}()
//
//	for j := 0; j < c; j++ {
//		jj := j
//		cmax := max / c
//		go func() {
//			for i := 0; i < cmax; i++ {
//				if rand.Intn(10) == 0 {
//					time.Sleep(time.Duration(rand.Intn(1000)))
//				}
//				q.Push(fmt.Sprintf("%v %v", jj, i))
//			}
//		}()
//	}
//
//	wg.Wait()
//	time.Sleep(500 * time.Millisecond)
//	// queue should be empty
//	for i := 0; i < 100; i++ {
//		r, ok := q.Pop()
//		if ok {
//			log.Printf("unexpected result %+v", r)
//			t.FailNow()
//		}
//	}
//}
