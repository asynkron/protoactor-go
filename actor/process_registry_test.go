package actor

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUint64ToId(t *testing.T) {
	cases := []struct {
		i uint64
		e string
	}{
		{0xfedcba9876543210, "$fXsKFxSl38g"},
		{0x0, "$0"},
		{0x1, "$1"},
		{0xf, "$f"},
		{0x1041041041041041, "$11111111111"},
	}
	for _, tc := range cases {
		t.Run(tc.e, func(t *testing.T) {
			s := uint64ToId(tc.i)
			assert.Equal(t, tc.e, s)
		})
	}
}

var ss string

func BenchmarkUint64ToId(b *testing.B) {
	var s string
	for i := 0; i < b.N; i++ {
		s = uint64ToId(uint64(i) << 5)
	}
	ss = s
}

func BenchmarkUint64ToString2(b *testing.B) {
	var s string
	var buf [12]byte
	for i := 0; i < b.N; i++ {
		s = string(strconv.AppendUint(buf[:], uint64(i)<<5, 36))
	}
	ss = s
}
