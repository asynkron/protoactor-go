package mailbox

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"testing"
	"time"
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
func (*invoker) EscalateFailure(reason interface{}, message interface{}) {}

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
	q := p(mi, NewDefaultDispatcher(300))

	for j := 0; j < c; j++ {
		cmax := max / c
		go func() {
			for i := 0; i < cmax; i++ {
				if rand.Intn(10) == 0 {
					time.Sleep(time.Duration(rand.Intn(1000)))
				}
				q.PostUserMessage(fmt.Sprintf("%v %v", j, i))
			}
		}()
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
	q := p(mi, NewDefaultDispatcher(300))

	for j := 0; j < c; j++ {
		cmax := max / c
		go func() {
			for i := 0; i < cmax; i++ {
				if rand.Intn(10) == 0 {
					time.Sleep(time.Duration(rand.Intn(100)))
				}
				q.PostSystemMessage(
					&sysDummy{
						value: fmt.Sprintf("%v %v", j, i),
					})
			}
		}()
	}
	wg.Wait()
	time.Sleep(1 * time.Second)
}
