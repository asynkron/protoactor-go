package actor

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBehaviorStack_Len(t *testing.T) {
	var bs behaviorStack
	assert.Len(t, bs, 0)
	bs.Push(func(Context) {})
	bs.Push(func(Context) {})
	assert.Len(t, bs, 2)
}

func TestBehaviorStack_Push(t *testing.T) {
	var bs behaviorStack
	assert.Len(t, bs, 0)
	bs.Push(func(Context) {})
	assert.Len(t, bs, 1)
	bs.Push(func(Context) {})
	assert.Len(t, bs, 2)
}

func TestBehaviorStack_Clear(t *testing.T) {
	var bs behaviorStack
	bs.Push(func(Context) {})
	bs.Push(func(Context) {})
	assert.Len(t, bs, 2)
	bs.Clear()
	assert.Len(t, bs, 0)
}

func TestBehaviorStack_Peek(t *testing.T) {
	called := 0
	fn1 := ActorFunc(func(Context) { called = 1 })
	fn2 := ActorFunc(func(Context) { called = 2 })

	cases := []struct {
		items    []ActorFunc
		expected int
	}{
		{[]ActorFunc{fn1, fn2}, 2},
		{[]ActorFunc{fn2, fn1}, 1},
	}

	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			var bs behaviorStack
			for _, fn := range tc.items {
				bs.Push(fn)
			}
			a, _ := bs.Peek()
			a(nil)
			assert.Equal(t, tc.expected, called)
		})
	}
}

func TestBehaviorStack_Pop_ExpectedOrder(t *testing.T) {
	called := 0
	fn1 := ActorFunc(func(Context) { called = 1 })
	fn2 := ActorFunc(func(Context) { called = 2 })

	cases := []struct {
		items    []ActorFunc
		expected []int
	}{
		{[]ActorFunc{fn1, fn2}, []int{2, 1}},
		{[]ActorFunc{fn2, fn1}, []int{1, 2}},
	}

	for i, tc := range cases {
		t.Run("order "+strconv.Itoa(i), func(t *testing.T) {
			var bs behaviorStack
			for _, fn := range tc.items {
				bs.Push(fn)
			}

			for _, e := range tc.expected {
				a, _ := bs.Pop()
				a(nil)
				assert.Equal(t, e, called)
				called = 0
			}
		})
	}
}
