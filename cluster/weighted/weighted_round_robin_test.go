package weighted

import "testing"

var wrr = NewWeightedRoundRobin(nil)

func BenchmarkGCD40(b *testing.B) {
	val := []int{4, 4, 8, 6, 12, 30, 50, 150, 124, 124, 52, 66, 68, 168, 190, 244, 690, 400, 120, 520, 4, 4, 8, 6, 12, 30, 50, 150, 124, 124, 52, 66, 68, 168, 190, 244, 690, 400, 120, 520}

	for n := 0; n < b.N; n++ {
		wrr.ngcd(val)
	}
}

func BenchmarkGCD20(b *testing.B) {
	val := []int{4, 4, 8, 6, 12, 30, 50, 150, 124, 124, 52, 66, 68, 168, 190, 244, 690, 400, 120, 520}

	for n := 0; n < b.N; n++ {
		wrr.ngcd(val)
	}
}

func BenchmarkGCD2(b *testing.B) {
	val := []int{4, 520}

	for n := 0; n < b.N; n++ {
		wrr.ngcd(val)
	}
}
