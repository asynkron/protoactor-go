package cluster

import (
	"fmt"
	"runtime"
	"testing"
)

func Benchmark_Rendezvous_Get(b *testing.B) {

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
				address := obj.GetByIdentity("kind/0123456789abcdefghijklmnopqrstuvwxyz")
				if address == "" {
					b.Fatalf("empty address res=%d", len(members))
				}
			}
		})
	}
}
