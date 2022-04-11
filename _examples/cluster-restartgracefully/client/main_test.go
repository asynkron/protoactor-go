package main

import "testing"

func BenchmarkCalcAdd(t *testing.B) {
	startNode(0, "consul")
	calcAdd("yes", 1)
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		calcAdd("yes", 1)
	}
}
