package actor

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"testing"
	"time"

	rbqueue "github.com/Workiva/go-datastructures/queue"

	"github.com/stretchr/testify/assert"
)

type invoker struct {
	count int
	max   int
	wg    *sync.WaitGroup
}

func (i *invoker) InvokeSystemMessage(interface{}) {
	i.count++
	if i.count == i.max {
		i.wg.Done()
	}
	if i.count > i.max {
		log.Println("Unexpected data..")
	}
}

func (i *invoker) InvokeUserMessage(interface{}) {
	i.count++
	if i.count == i.max {
		i.wg.Done()
	}
	if i.count > i.max {
		log.Println("Unexpected data..")
	}
}

func (*invoker) EscalateFailure(_ interface{}, _ interface{}) {}

func TestUnboundedLockfreeMailboxUsermessageConsistency(t *testing.T) {
	max := 1000000
	c := 100
	var wg sync.WaitGroup
	wg.Add(1)
	p := UnboundedLockfree()
	mi := &invoker{
		max: max,
		wg:  &wg,
	}
	q := p()
	q.RegisterHandlers(mi, NewDefaultDispatcher(300))

	for j := 0; j < c; j++ {
		cmax := max / c
		go func(j int) {
			for i := 0; i < cmax; i++ {
				if rand.Intn(10) == 0 {
					time.Sleep(time.Duration(rand.Intn(1000)))
				}
				q.PostUserMessage(fmt.Sprintf("%v %v", j, i))
			}
		}(j)
	}
	wg.Wait()
	time.Sleep(1 * time.Second)
}

type sysDummy struct {
	value string
}

func (*sysDummy) SystemMessage() {
}

func TestUnboundedLockfreeMailboxSysMessageConsistency(t *testing.T) {
	max := 1000000
	c := 100
	var wg sync.WaitGroup
	wg.Add(1)
	p := UnboundedLockfree()
	mi := &invoker{
		max: max,
		wg:  &wg,
	}
	q := p()
	q.RegisterHandlers(mi, NewDefaultDispatcher(300))

	for j := 0; j < c; j++ {
		cmax := max / c
		go func(j int) {
			for i := 0; i < cmax; i++ {
				if rand.Intn(10) == 0 {
					time.Sleep(time.Duration(rand.Intn(100)))
				}
				q.PostSystemMessage(
					&sysDummy{
						value: fmt.Sprintf("%v %v", j, i),
					})
			}
		}(j)
	}
	wg.Wait()
	time.Sleep(1 * time.Second)
}

func TestBoundedMailbox(t *testing.T) {
	size := 3
	m := boundedMailboxQueue{
		userMailbox: rbqueue.NewRingBuffer(uint64(size)),
		dropping:    false,
	}
	m.Push("1")
	m.Push("2")
	m.Push("3")
	assert.Equal(t, "1", m.Pop())
}

func TestBoundedDroppingMailbox(t *testing.T) {
	size := 3
	m := boundedMailboxQueue{
		userMailbox: rbqueue.NewRingBuffer(uint64(size)),
		dropping:    true,
	}
	m.Push("1")
	m.Push("2")
	m.Push("3")
	m.Push("4")
	assert.Equal(t, "2", m.Pop())
}

func TestMailboxUserMessageCount(t *testing.T) {
	max := 10
	c := 10
	var wg sync.WaitGroup
	wg.Add(1)
	p := UnboundedLockfree()
	mi := &invoker{
		max: max,
		wg:  &wg,
	}
	q := p()
	q.RegisterHandlers(mi, NewDefaultDispatcher(300))

	for j := 0; j < c; j++ {
		q.PostUserMessage(fmt.Sprintf("%v", j))
	}
	assert.Equal(t, c, q.UserMessageCount())
	wg.Wait()
	time.Sleep(100 * time.Millisecond)
}
