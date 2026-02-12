package actor

import (
	"errors"
	"sync"
)

// roundUpPowerOf2 rounds v up to the next power of 2.
// This matches the behavior of the go-datastructures RingBuffer
// which the bounded mailbox logic depends on (Cap > requested size).
func roundUpPowerOf2(v int) int {
	if v <= 0 {
		return 1
	}
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v |= v >> 32
	v++
	return v
}

// ringBuffer is a thread-safe fixed-capacity FIFO ring buffer for mailbox messages.
// It provides a similar API to the go-datastructures RingBuffer it replaces.
// The capacity is rounded up to the next power of 2, matching the behavior
// of the original implementation.
type ringBuffer struct {
	buf  []interface{}
	head int
	tail int
	size int
	cap  int
	mu   sync.Mutex
	cond *sync.Cond
}

func newRingBuffer(capacity int) *ringBuffer {
	capacity = roundUpPowerOf2(capacity)
	rb := &ringBuffer{
		buf: make([]interface{}, capacity),
		cap: capacity,
	}
	rb.cond = sync.NewCond(&rb.mu)
	return rb
}

// Put adds an item to the ring buffer. If the buffer is full, it blocks until
// space becomes available.
func (rb *ringBuffer) Put(item interface{}) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	for rb.size == rb.cap {
		rb.cond.Wait()
	}
	rb.buf[rb.tail] = item
	rb.tail = (rb.tail + 1) % rb.cap
	rb.size++
	rb.cond.Signal()
	return nil
}

// Get removes and returns the oldest item. If the buffer is empty, it returns
// an error. Callers should check Len() before calling Get() to avoid errors.
func (rb *ringBuffer) Get() (interface{}, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.size == 0 {
		return nil, errors.New("ring buffer is empty")
	}
	item := rb.buf[rb.head]
	rb.buf[rb.head] = nil // clear reference for GC
	rb.head = (rb.head + 1) % rb.cap
	rb.size--
	rb.cond.Signal()
	return item, nil
}

// Len returns the number of items currently in the buffer.
func (rb *ringBuffer) Len() uint64 {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return uint64(rb.size)
}

// Cap returns the capacity of the buffer.
func (rb *ringBuffer) Cap() uint64 {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return uint64(rb.cap)
}
