// Package mpsc provides an efficient implementation of a multi-producer, single-consumer lock-free queue.
//
// The Push function is safe to call from multiple goroutines. The Pop and Empty APIs must only be
// called from a single, consumer goroutine.
//
package mpsc

// This implementation is based on http://www.1024cores.net/home/lock-free-algorithms/queues/non-intrusive-mpsc-node-based-queue

import (
	"sync/atomic"
	"unsafe"
)

type node struct {
	next *node
	val  interface{}
}

type Queue struct {
	head, tail *node
	stub       node
}

func New() *Queue {
	q := &Queue{}
	q.head = &q.stub
	q.tail = q.head
	return q
}

// Push adds x to the back of the queue.
//
// Push can be safely called from multiple goroutines
func (q *Queue) Push(x interface{}) {
	n := &node{val: x}
	// current producer acquires head node
	prev := (*node)(unsafe.Pointer(atomic.SwapUintptr((*uintptr)(unsafe.Pointer(&q.head)), (uintptr)(unsafe.Pointer(n)))))

	// release node to consumer
	prev.next = n
}

// Pop removes the item from the front of the queue or nil if the queue is empty
//
// Pop must be called from a single, consumer goroutine
func (q *Queue) Pop() interface{} {
	tail := q.tail
	next := (*node)(unsafe.Pointer(atomic.LoadUintptr((*uintptr)(unsafe.Pointer(&tail.next))))) // acquire
	if next != nil {
		q.tail = next
		return next.val
	}
	return nil
}

// Empty returns true if the queue is empty
//
// Empty must be called from a single, consumer goroutine
func (q *Queue) Empty() bool {
	tail := q.tail
	next := atomic.LoadUintptr((*uintptr)(unsafe.Pointer(&tail.next)))
	return next == 0
}
