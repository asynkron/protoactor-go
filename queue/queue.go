package queue

import "sync"

type Queue struct {
	buffer []interface{}
	head   int
	tail   int
	len    int
	mod    int
	lock   sync.RWMutex
}

func New() *Queue {
	return &Queue{
		buffer: make([]interface{}, 100),
		head:   0,
		tail:   0,
		len:    0,
		mod:    100,
	}
}

func (q *Queue) Push(item interface{}) {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.tail = ((q.tail + 1) % q.mod)
	if q.tail == q.head {
		//we need to resize
		newBuff := make([]interface{}, q.mod*2)
		for i := 0; i < q.mod; i++ {
			buffIndex := (q.head + i) % q.mod
			x := q.buffer[buffIndex]
			newBuff[i] = x
		}
		q.buffer = newBuff
		q.head = 0
		q.tail = q.mod
		q.mod *= 2
	}
	q.len++
	q.buffer[q.tail] = item
}

func (q *Queue) Length() int {
	q.lock.RLock()
	defer q.lock.RUnlock()

	return q.len
}

func (q *Queue) Empty() bool {
	return q.Length() == 0
}

func (q *Queue) Pop() interface{} {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.head = ((q.head + 1) % q.mod)
	if q.len == 0 {

		return nil
	}
	q.len--
	return q.buffer[q.head]
}
