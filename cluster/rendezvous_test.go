package cluster

import (
	"fmt"
	"runtime"
	"sync"
	"testing"


	"github.com/asynkron/protoactor-go/log"
)

func Test_Rendezvous_Parallel_Get(t *testing.T) {

	const testIdCount = 10
	const testRepeatCount = 10000

	obj := NewRendezvous()
	members := newMembersForTest(testIdCount)
	ms := newDefaultMemberStrategy(nil, "kind").(*simpleMemberStrategy)
	for _, member := range members {
		ms.AddMember(member)
	}
	obj.UpdateMembers(members)

	var wg sync.WaitGroup
	var once sync.Once

	var failureMsg string
	failFn := func(msg string) func() {
		return func() {
			failureMsg = msg
		}
	}

	wg.Add(testIdCount)

	for i := 0; i < testIdCount; i++ {
		id := fmt.Sprintf("0123456789abcdefghijklmnopqrstuvwxyz%d", i)
		addr := obj.GetByIdentity(id)

		go func() {
			for i := 0; i < testRepeatCount; i++ {
				rstAddr := obj.GetByIdentity(id)
				if addr != rstAddr {
					// send fail signal
					once.Do(failFn(fmt.Sprintf("address should be consistent for same id. previous address:%s address:%s id:%s", addr, rstAddr, id)))
					break
				}
			}
			wg.Done()
		}()
	}

	wg.Wait()
	if failureMsg != "" {
		t.Fatal(failureMsg)
	}
}


func Benchmark_Rendezvous_Get(b *testing.B) {
	SetLogLevel(log.ErrorLevel)
	for _, v := range []int{1, 2, 3, 5, 10, 100, 1000, 2000} {
		members := newMembersForTest(v)
		ms := newDefaultMemberStrategy(nil, "kind").(*simpleMemberStrategy)
		for _, member := range members {
			ms.AddMember(member)
		}
		obj := NewRendezvous()
		obj.UpdateMembers(members)
		testName := fmt.Sprintf("member*%d", v)
		runtime.GC()
		b.Run(testName, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				address := obj.GetByIdentity("0123456789abcdefghijklmnopqrstuvwxyz")
				if address == "" {
					b.Fatalf("empty address res=%d", len(members))
				}
			}
		})
	}
}
