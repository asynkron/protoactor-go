package mailbox

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AsynkronIT/protoactor-go/internal/queue/goring"
	"github.com/AsynkronIT/protoactor-go/internal/queue/mpsc"
)

type Message interface {
	GetMessage() string
}

type TestPriorityMessage struct {
	message  string
	priority int8
}

type TestMessage struct {
	message string
}

func (tpm *TestPriorityMessage) GetPriority() int8 {
	return tpm.priority
}

func (tpm *TestPriorityMessage) GetMessage() string {
	return tpm.message
}

func (tm *TestMessage) GetMessage() string {
	return tm.message
}

func NewTestGoringPriorityQueue() *priorityQueue {
	return NewPriorityQueue(func() queue {
		return &unboundedMailboxQueue{
			userMailbox: goring.New(1),
		}
	})
}

func NewTestMpscPriorityQueue() *priorityQueue {
	return NewPriorityQueue(func() queue {
		return mpsc.New()
	})
}

func TestPushPopGoring(t *testing.T) {
	q := NewTestGoringPriorityQueue()
	q.Push("hello")
	res := q.Pop()
	assert.Equal(t, "hello", res)
}

func TestPushPopGoringPriority(t *testing.T) {
	q := NewTestGoringPriorityQueue()

	// pushes

	for i := 0; i < 2; i++ {
		q.Push(&TestPriorityMessage{
			message:  "7 hello",
			priority: 7,
		})
	}

	for i := 0; i < 2; i++ {
		q.Push(&TestPriorityMessage{
			message:  "5 hello",
			priority: 5,
		})
	}

	for i := 0; i < 2; i++ {
		q.Push(&TestPriorityMessage{
			message:  "0 hello",
			priority: 0,
		})
	}

	for i := 0; i < 2; i++ {
		q.Push(&TestPriorityMessage{
			message:  "6 hello",
			priority: 6,
		})
	}

	for i := 0; i < 2; i++ {
		q.Push(&TestMessage{message: "hello"})
	}

	// pops in priority order

	for i := 0; i < 2; i++ {
		res := q.Pop()
		assert.Equal(t, "7 hello", res.(Message).GetMessage())
	}

	for i := 0; i < 2; i++ {
		res := q.Pop()
		assert.Equal(t, "6 hello", res.(Message).GetMessage())
	}

	for i := 0; i < 2; i++ {
		res := q.Pop()
		assert.Equal(t, "5 hello", res.(Message).GetMessage())
	}

	for i := 0; i < 2; i++ {
		res := q.Pop()
		assert.Equal(t, "hello", res.(Message).GetMessage())
	}

	for i := 0; i < 2; i++ {
		res := q.Pop()
		assert.Equal(t, "0 hello", res.(Message).GetMessage())
	}
}

func TestPushPopMpsc(t *testing.T) {
	q := NewTestMpscPriorityQueue()
	q.Push("hello")
	res := q.Pop()
	assert.Equal(t, "hello", res)
}

func TestPushPopMpscPriority(t *testing.T) {
	q := NewTestMpscPriorityQueue()

	// pushes

	for i := 0; i < 2; i++ {
		q.Push(&TestPriorityMessage{
			message:  "7 hello",
			priority: 7,
		})
	}

	for i := 0; i < 2; i++ {
		q.Push(&TestPriorityMessage{
			message:  "5 hello",
			priority: 5,
		})
	}

	for i := 0; i < 2; i++ {
		q.Push(&TestPriorityMessage{
			message:  "0 hello",
			priority: 0,
		})
	}

	for i := 0; i < 2; i++ {
		q.Push(&TestPriorityMessage{
			message:  "6 hello",
			priority: 6,
		})
	}

	for i := 0; i < 2; i++ {
		q.Push(&TestMessage{message: "hello"})
	}

	// pops in priority order

	for i := 0; i < 2; i++ {
		res := q.Pop()
		assert.Equal(t, "7 hello", res.(Message).GetMessage())
	}

	for i := 0; i < 2; i++ {
		res := q.Pop()
		assert.Equal(t, "6 hello", res.(Message).GetMessage())
	}

	for i := 0; i < 2; i++ {
		res := q.Pop()
		assert.Equal(t, "5 hello", res.(Message).GetMessage())
	}

	for i := 0; i < 2; i++ {
		res := q.Pop()
		assert.Equal(t, "hello", res.(Message).GetMessage())
	}

	for i := 0; i < 2; i++ {
		res := q.Pop()
		assert.Equal(t, "0 hello", res.(Message).GetMessage())
	}
}
