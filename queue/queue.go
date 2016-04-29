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
	initialSize := 10
	return &Queue{
		buffer: make([]interface{}, initialSize),
		head:   0,
		tail:   0,
		len:    0,
		mod:    initialSize,
	}
}

func (q *Queue) Push(item interface{}) {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.tail = ((q.tail + 1) % q.mod)
	if q.tail == q.head {
		fillFactor := 2
		//we need to resize

		newLen := q.mod * fillFactor
		newBuff := make([]interface{}, newLen)

		for i := 0; i < q.mod; i++ {
			buffIndex := (q.head + i) % q.mod
			newBuff[i] = q.buffer[buffIndex]
		}
		//set the new buffer and reset head and tail
		q.buffer = newBuff
		q.head = 0
		q.tail = q.mod
		q.mod *= fillFactor
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

func (q *Queue) Pop() (interface{}, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if q.len == 0 {

		return nil, false
	}
	q.head = ((q.head + 1) % q.mod)
	q.len--
	return q.buffer[q.head], true
}

func (q *Queue) PopMany(count int) ([]interface{}, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.len == 0 {
		return nil, false
	}

	if count >= q.len {
		count = q.len
	}

	buffer := make([]interface{}, count)
	for i := 0; i < count; i++ {
		buffer[i] = q.buffer[(q.head+1+i)%q.mod]
	}
	q.head = (q.head + count) % q.mod
	q.len -= count
	return buffer, true
}
