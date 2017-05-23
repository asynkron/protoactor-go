package gocbcore

import "sync"

// This is used to store operations while they are pending
//   a response from the server to allow mapping of a response
//   opaque back to the originating request.  This queue takes
//   advantage of the monotonic nature of the opaque values
//   and synchronous responses from the server to nearly always
//   return the request without needing to iterate at all.
type memdOpMap struct {
	lock    sync.Mutex
	opIndex uint32

	first *memdQRequest
	last  *memdQRequest
}

// Add a new request to the bottom of the op queue.
func (q *memdOpMap) Add(r *memdQRequest) {
	q.lock.Lock()

	q.opIndex++
	r.Opaque = q.opIndex
	r.queueNext = nil

	if q.last == nil {
		q.first = r
		q.last = r
	} else {
		q.last.queueNext = r
		q.last = r
	}

	q.lock.Unlock()
}

// Removes a request from the op queue.  Expects to be passed
//   the request to remove, along with the request that
//   immediately preceeds it in the queue.
func (q *memdOpMap) remove(prev *memdQRequest, req *memdQRequest) {
	if prev == nil {
		q.first = req.queueNext
		if q.first == nil {
			q.last = nil
		}
		return
	}
	prev.queueNext = req.queueNext
	if prev.queueNext == nil {
		q.last = prev
	}
}

// Removes a specific request from the op queue.
func (q *memdOpMap) Remove(req *memdQRequest) bool {
	q.lock.Lock()

	var cur *memdQRequest = q.first
	var prev *memdQRequest
	for cur != nil {
		if cur == req {
			q.remove(prev, cur)
			q.lock.Unlock()
			return true
		}
		prev = cur
		cur = cur.queueNext
	}

	q.lock.Unlock()
	return false
}

// Locates a request (searching FIFO-style) in the op queue using
//   the opaque value that was assigned to it when it was dispatched.
//   It then removes the request from the queue if it is not persistent
//   or if alwaysRemove is set to true.
func (q *memdOpMap) FindAndMaybeRemove(opaque uint32, alwaysRemove bool) *memdQRequest {
	q.lock.Lock()

	var cur *memdQRequest = q.first
	var prev *memdQRequest
	for cur != nil {
		if cur.Opaque == opaque {
			if !cur.Persistent || alwaysRemove {
				q.remove(prev, cur)
			}

			q.lock.Unlock()
			return cur
		}
		prev = cur
		cur = cur.queueNext
	}

	q.lock.Unlock()
	return nil
}

// Clears the queue of all requests and calls the passed function
//   once for each request found in the queue.
func (q *memdOpMap) Drain(cb func(*memdQRequest)) {
	for cur := q.first; cur != nil; cur = cur.queueNext {
		cb(cur)
	}
	q.first = nil
	q.last = nil
}
