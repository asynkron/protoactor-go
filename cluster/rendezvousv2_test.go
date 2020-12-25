package cluster

import (
	"fmt"
	"runtime"
	"testing"
)

func Benchmark_RendezvousV2_Get(b *testing.B) {
	for _, v := range []int{1, 2, 3, 5, 10, 100, 1000, 2000} {
		members := _newTopologyEventForTest(v)
		obj := NewRendezvousV2(members)
		testName := fmt.Sprintf("member*%d", v)
		runtime.GC()
		b.Run(testName, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				address := obj.Get("0123456789abcdefghijklmnopqrstuvwxyz")
				if address == "" {
					b.Fatalf("empty address res=%d", len(members))
				}
			}
		})
	}
}
