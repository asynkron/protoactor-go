package log

import "testing"

func Benchmark_OffLevel(b *testing.B) {
	l := New(MinLevel, "")
	for i := 0; i < b.N; i++ {
		l.Debug("foo", Int("bar", 32), Bool("fum", false))
	}
}
