package goring

import (
	"sync"
	"sync/atomic"
)

type ringBuffer struct {
	buffer []interface{}
	head   int64
	tail   int64
	mod    int64
}

type Queue struct {
	content *ringBuffer
	len     int64
	lock    sync.Mutex
}

func New(initialSize int64) *Queue {
	return &Queue{
		content: &ringBuffer{
			buffer: make([]interface{}, initialSize),
			head:   0,
			tail:   0,
			mod:    initialSize,
		},
		len: 0,
	}
}

func (q *Queue) Push(item interface{}) {
	q.lock.Lock()
	c := q.content
	c.tail = ((c.tail + 1) % c.mod)
	if c.tail == c.head {
		var fillFactor int64 = 10
		//we need to resize

		newLen := c.mod * fillFactor
		newBuff := make([]interface{}, newLen)

		for i := int64(0); i < c.mod; i++ {
			buffIndex := (c.tail + i) % c.mod
			newBuff[i] = c.buffer[buffIndex]
		}
		//set the new buffer and reset head and tail
		newContent := &ringBuffer{
			buffer: newBuff,
			head:   0,
			tail:   c.mod,
			mod:    c.mod * fillFactor,
		}
		q.content = newContent
	}
	q.len++
	q.content.buffer[q.content.tail] = item
	q.lock.Unlock()
}

func (q *Queue) Length() int64 {
	res := atomic.LoadInt64(&q.len)
	return res
}

func (q *Queue) Empty() bool {
	return q.Length() == 0
}

//single consumer
func (q *Queue) Pop() (interface{}, bool) {

	if q.Empty() {
		return nil, false
	}
	//as we are a single consumer, no other thread can have poped the items there are guaranteed to be items now
	q.lock.Lock()
	c := q.content
	c.head = ((c.head + 1) % c.mod)
	res := c.buffer[c.head]
	q.len--
	q.lock.Unlock()
	return res, true
}

func (q *Queue) PopMany(count int64) ([]interface{}, bool) {

	if q.Empty() {
		return nil, false
	}

	q.lock.Lock()
	c := q.content

	if count >= q.len {
		count = q.len
	}
	q.len -= count

	buffer := make([]interface{}, count)
	for i := int64(0); i < count; i++ {
		v := c.buffer[(c.head+1+i)%c.mod]
		buffer[i] = v
	}
	c.head = (c.head + count) % c.mod

	q.lock.Unlock()
	return buffer, true
}
