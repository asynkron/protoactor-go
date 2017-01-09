package actor

import (
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
	s.Add(NewLocalPID("p1"))
	s.Add(NewLocalPID("p2"))
	s.Add(NewLocalPID("p3"))
	assert.Equal(t, 3, s.Len())
	s.Clear()
	assert.True(t, s.Empty())
	assert.Len(t, s.s, 0)
}

func TestPIDSet_AddSmall(t *testing.T) {
	var s PIDSet
	p1 := NewLocalPID("p1")
	s.Add(p1)
	assert.False(t, s.Empty())
	p1 = NewLocalPID("p1")
	s.Add(p1)
	assert.Equal(t, 1, s.Len())
}

func TestPIDSet_Values(t *testing.T) {
	var s PIDSet
	s.Add(NewLocalPID("p1"))
	s.Add(NewLocalPID("p2"))
	s.Add(NewLocalPID("p3"))
	assert.False(t, s.Empty())

	r := s.Values()
	assert.Len(t, r, 3)
}

func TestPIDSet_AddMap(t *testing.T) {
	var s PIDSet
	s.m = make(map[string]struct{})
	p1 := NewLocalPID("p1")
	s.Add(p1)
	assert.False(t, s.Empty())
	p1 = NewLocalPID("p1")
	s.Add(p1)
	assert.Equal(t, 1, s.Len())
}

var pids []*PID

func init() {
	for i := 0; i < 1000; i++ {
		pids = append(pids, NewLocalPID("p"+strconv.Itoa(i)))
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
