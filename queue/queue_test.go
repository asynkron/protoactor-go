package queue

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPushPop(t *testing.T) {
	q := New()
	q.Push("hello")
	res, _ := q.Pop()
	assert.Equal(t, "hello", res)
	assert.True(t, q.Empty())
}

func TestPushPopMany(t *testing.T) {
	q := New()
	for i := 0; i < 10000; i++ {
		item := fmt.Sprintf("hello%v", i)
		q.Push(item)
		res, _ := q.Pop()
		assert.Equal(t, item, res)
	}
	assert.True(t, q.Empty())
}

func TestPushPopMany2(t *testing.T) {
	q := New()
	for i := 0; i < 10000; i++ {
		item := fmt.Sprintf("hello%v", i)
		q.Push(item)
	}
	for i := 0; i < 10000; i++ {
		item := fmt.Sprintf("hello%v", i)
		res, _ := q.Pop()
		assert.Equal(t, item, res)
	}
	assert.True(t, q.Empty())
}
