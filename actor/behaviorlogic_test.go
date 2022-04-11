package actor

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBehavior_Len(t *testing.T) {
	var bs Behavior

	assert.Len(t, bs, 0)
	bs.push(func(Context) {})
	bs.push(func(Context) {})
	assert.Len(t, bs, 2)
}

func TestBehavior_Push(t *testing.T) {
	var bs Behavior

	assert.Len(t, bs, 0)
	bs.push(func(Context) {})
	assert.Len(t, bs, 1)
	bs.push(func(Context) {})
	assert.Len(t, bs, 2)
}

func TestBehavior_Clear(t *testing.T) {
	var bs Behavior

	bs.push(func(Context) {})
	bs.push(func(Context) {})
	assert.Len(t, bs, 2)
	bs.clear()
	assert.Len(t, bs, 0)
}

func TestBehavior_Peek(t *testing.T) {
	called := 0
	fn1 := ReceiveFunc(func(Context) { called = 1 })
	fn2 := ReceiveFunc(func(Context) { called = 2 })

	cases := []struct {
		items    []ReceiveFunc
		expected int
	}{
		{[]ReceiveFunc{fn1, fn2}, 2},
		{[]ReceiveFunc{fn2, fn1}, 1},
	}

	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			var bs Behavior
			for _, fn := range tc.items {
				bs.push(fn)
			}
			a, _ := bs.peek()
			a(nil)
			assert.Equal(t, tc.expected, called)
		})
	}
}

func TestBehaviorStack_Pop_ExpectedOrder(t *testing.T) {
	called := 0
	fn1 := ReceiveFunc(func(Context) { called = 1 })
	fn2 := ReceiveFunc(func(Context) { called = 2 })

	cases := []struct {
		items    []ReceiveFunc
		expected []int
	}{
		{[]ReceiveFunc{fn1, fn2}, []int{2, 1}},
		{[]ReceiveFunc{fn2, fn1}, []int{1, 2}},
	}

	for i, tc := range cases {
		t.Run("order "+strconv.Itoa(i), func(t *testing.T) {
			var bs Behavior
			for _, fn := range tc.items {
				bs.push(fn)
			}

			for _, e := range tc.expected {
				a, _ := bs.pop()
				a(nil)
				assert.Equal(t, e, called)
				called = 0
			}
		})
	}
}
