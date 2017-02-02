package lfqueue

// Author: https://github.com/antigloss

// Package queue offers goroutine-safe Queue implementations such as LockfreeQueue(Lock free queue).

import (
	"sync/atomic"
	"unsafe"
)

// NewLockfreeQueue is the only way to get a new, ready-to-use LockfreeQueue.
//
// Example:
//
//   lfq := queue.NewLockfreeQueue()
//   lfq.Push(100)
//   v := lfq.Pop()
func NewLockfreeQueue() *LockfreeQueue {
	var lfq LockfreeQueue
	lfq.head = unsafe.Pointer(&lfqNode{})
	lfq.tail = lfq.head
	return &lfq
}

// LockfreeQueue is a goroutine-safe Queue implementation.
// The overall performance of LockfreeQueue is much better than List+Mutex(standard package).
type LockfreeQueue struct {
	head unsafe.Pointer
	tail unsafe.Pointer
}

// Pop returns (and removes) an element from the front of the queue, or nil if the queue is empty.
// It performs about 100% better than list.List.Front() and list.List.Remove() with sync.Mutex.
func (lfq *LockfreeQueue) Pop() interface{} {
	for {
		h := atomic.LoadPointer(&lfq.head)
		rh := (*lfqNode)(h)
		n := (*lfqNode)(atomic.LoadPointer(&rh.next))
		if n != nil {
			if atomic.CompareAndSwapPointer(&lfq.head, h, rh.next) {
				v := n.val
				n.val = nil
				return v
			} else {
				continue
			}
		} else {
			return nil
		}
	}
}

// Push inserts an element to the back of the queue.
// It performs exactly the same as list.List.PushBack() with sync.Mutex.
func (lfq *LockfreeQueue) Push(val interface{}) {
	node := unsafe.Pointer(&lfqNode{val: val})
	for {
		t := atomic.LoadPointer(&lfq.tail)
		rt := (*lfqNode)(t)
		if atomic.CompareAndSwapPointer(&rt.next, nil, node) {
			// It'll be a dead loop if atomic.StorePointer() is used.
			// Don't know why.
			// atomic.StorePointer(&lfq.tail, node)
			atomic.CompareAndSwapPointer(&lfq.tail, t, node)
			return
		} else {
			continue
		}
	}
}

type lfqNode struct {
	val  interface{}
	next unsafe.Pointer
}
