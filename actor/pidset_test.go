package actor

import (
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPIDSet_Empty(t *testing.T) {
	var s PIDSet
	assert.True(t, s.Empty())
}

func TestPIDSet_Clear(t *testing.T) {
	var s PIDSet
	s.Add(NewPID(localAddress, "p1"))
	s.Add(NewPID(localAddress, "p2"))
	s.Add(NewPID(localAddress, "p3"))
	assert.Equal(t, 3, s.Len())
	s.Clear()
	assert.True(t, s.Empty())
	assert.Len(t, s.pids, 0)
}

func TestPIDSet_Remove(t *testing.T) {
	var s PIDSet
	s.Add(NewPID(localAddress, "p1"))
	s.Add(NewPID(localAddress, "p2"))
	s.Add(NewPID(localAddress, "p3"))
	assert.Equal(t, 3, s.Len())

	s.Remove(NewPID(localAddress, "p3"))
	assert.Equal(t, 2, s.Len())
	assert.False(t, s.Contains(NewPID(localAddress, "p3")))
}

func TestPIDSet_AddSmall(t *testing.T) {
	s := NewPIDSet()
	p1 := NewPID(localAddress, "p1")
	s.Add(p1)
	assert.False(t, s.Empty())
	p1 = NewPID(localAddress, "p1")
	s.Add(p1)
	assert.Equal(t, 1, s.Len())
}

func TestPIDSet_Values(t *testing.T) {
	var s PIDSet
	s.Add(NewPID(localAddress, "p1"))
	s.Add(NewPID(localAddress, "p2"))
	s.Add(NewPID(localAddress, "p3"))
	assert.False(t, s.Empty())

	r := s.Values()
	assert.Len(t, r, 3)
}

func TestPIDSet_AddMap(t *testing.T) {
	s := NewPIDSet()
	p1 := NewPID(localAddress, "p1")
	s.Add(p1)
	assert.False(t, s.Empty())
	p1 = NewPID(localAddress, "p1")
	s.Add(p1)
	assert.Equal(t, 1, s.Len())
}

var pids []*PID

func init() {
	for i := 0; i < 100000; i++ {
		pids = append(pids, NewPID(localAddress, "p"+strconv.Itoa(i)))
	}
}

func BenchmarkPIDSet_Add(b *testing.B) {
	cases := []struct {
		l int
	}{
		{l: 1},
		{l: 5},
		{l: 20},
		{l: 500},
	}

	for _, tc := range cases {
		b.Run("len "+strconv.Itoa(tc.l), func(b *testing.B) {
			pidSetAdd(b, pids[:tc.l])
		})
	}
}

func pidSetAdd(b *testing.B, data []*PID) {
	for i := 0; i < b.N; i++ {
		var s PIDSet
		for j := 0; j < len(data); j++ {
			s.Add(data[j])
		}
	}
}

func BenchmarkPIDSet_AddRemove(b *testing.B) {
	cases := []struct {
		l int
	}{
		{l: 1},
		{l: 5},
		{l: 20},
		{l: 500},
	}

	for _, tc := range cases {
		b.Run("len "+strconv.Itoa(tc.l), func(b *testing.B) {
			pidSetAddRemove(b, pids[:tc.l])
		})
	}
}

func pidSetAddRemove(b *testing.B, data []*PID) {
	for i := 0; i < b.N; i++ {
		var s PIDSet
		for j := 0; j < len(data); j++ {
			s.Add(data[j])
		}
		for j := 0; j < len(data); j++ {
			s.Remove(data[j])
		}
	}
}

func BenchmarkPIDSet(b *testing.B) {
	cases := []struct {
		l int
	}{
		{l: 1},
		{l: 5},
		{l: 20},
		{l: 500},
		{l: 1000},
		{l: 10000},
		{l: 100000},
	}

	for _, tc := range cases {
		b.Run("len "+strconv.Itoa(tc.l), func(b *testing.B) {
			b.StopTimer()
			var s PIDSet
			for i := 0; i < tc.l; i++ {
				s.Add(pids[i])
			}
			b.StartTimer()

			for i := 0; i < b.N; i++ {
				pid := pids[rand.Intn(len(pids))]

				s.Add(pid)

				s.Remove(s.Get(rand.Intn(s.Len())))
			}
		})
	}
}
