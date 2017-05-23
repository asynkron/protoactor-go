package gocb

import (
	"gopkg.in/couchbase/gocbcore.v2"
)

// Retrieves a document from the bucket
func (b *Bucket) Get(key string, valuePtr interface{}) (Cas, error) {
	return b.get(key, valuePtr)
}

// Retrieves a document and simultaneously updates its expiry time.
func (b *Bucket) GetAndTouch(key string, expiry uint32, valuePtr interface{}) (Cas, error) {
	return b.getAndTouch(key, expiry, valuePtr)
}

// Locks a document for a period of time, providing exclusive RW access to it.
func (b *Bucket) GetAndLock(key string, lockTime uint32, valuePtr interface{}) (Cas, error) {
	return b.getAndLock(key, lockTime, valuePtr)
}

// Unlocks a document which was locked with GetAndLock.
func (b *Bucket) Unlock(key string, cas Cas) (Cas, error) {
	cas, _, err := b.unlock(key, cas)
	return cas, err
}

// Returns the value of a particular document from a replica server.
func (b *Bucket) GetReplica(key string, valuePtr interface{}, replicaIdx int) (Cas, error) {
	return b.getReplica(key, valuePtr, replicaIdx)
}

// Touches a document, specifying a new expiry time for it.
func (b *Bucket) Touch(key string, cas Cas, expiry uint32) (Cas, error) {
	cas, _, err := b.touch(key, cas, expiry)
	return cas, err
}

// Removes a document from the bucket.
func (b *Bucket) Remove(key string, cas Cas) (Cas, error) {
	cas, _, err := b.remove(key, cas)
	return cas, err
}

// Inserts or replaces a document in the bucket.
func (b *Bucket) Upsert(key string, value interface{}, expiry uint32) (Cas, error) {
	cas, _, err := b.upsert(key, value, expiry)
	return cas, err
}

// Inserts a new document to the bucket.
func (b *Bucket) Insert(key string, value interface{}, expiry uint32) (Cas, error) {
	cas, _, err := b.insert(key, value, expiry)
	return cas, err
}

// Replaces a document in the bucket.
func (b *Bucket) Replace(key string, value interface{}, cas Cas, expiry uint32) (Cas, error) {
	cas, _, err := b.replace(key, value, cas, expiry)
	return cas, err
}

// Appends a string value to a document.
func (b *Bucket) Append(key, value string) (Cas, error) {
	cas, _, err := b.append(key, value)
	return cas, err
}

// Prepends a string value to a document.
func (b *Bucket) Prepend(key, value string) (Cas, error) {
	cas, _, err := b.prepend(key, value)
	return cas, err
}

// Performs an atomic addition or subtraction for an integer document.  Passing a
//  non-negative `initial` value will cause the document to be created if it did
//  not already exist.
func (b *Bucket) Counter(key string, delta, initial int64, expiry uint32) (uint64, Cas, error) {
	val, cas, _, err := b.counter(key, delta, initial, expiry)
	return val, cas, err
}

// ServerStats is a tree of statistics information returned from the server.
// stats := cb.Stats(...)
// for server := stats {
//    for statName, stat := server {
//      //...
//    }
// }
type ServerStats map[string]map[string]string

// Stats returns various server statistics from the cluster.
func (b *Bucket) Stats(key string) (statsOut ServerStats, errOut error) {
	signal := make(chan bool, 1)
	statsOut = make(ServerStats)

	op, errOut := b.client.Stats(key, func(stats map[string]gocbcore.SingleServerStats) {
		for curServer, curStats := range stats {
			if curStats.Error != nil && errOut == nil {
				errOut = curStats.Error
			}
			statsOut[curServer] = curStats.Stats
		}
		signal <- true
	})

	timeoutTmr := gocbcore.AcquireTimer(b.opTimeout)
	select {
	case <-signal:
		gocbcore.ReleaseTimer(timeoutTmr, false)
		return
	case <-timeoutTmr.C:
		gocbcore.ReleaseTimer(timeoutTmr, true)
		if !op.Cancel() {
			<-signal
			return
		}
		return nil, ErrTimeout
	}
}

type ioGetCallback gocbcore.GetCallback
type ioCasCallback gocbcore.StoreCallback
type ioCtrCallback gocbcore.CounterCallback

type hlpGetHandler func(ioGetCallback) (pendingOp, error)

func (b *Bucket) hlpGetExec(valuePtr interface{}, execFn hlpGetHandler) (casOut Cas, errOut error) {
	signal := make(chan bool, 1)
	op, err := execFn(func(bytes []byte, flags uint32, cas gocbcore.Cas, err error) {
		errOut = err
		if errOut == nil {
			errOut = b.transcoder.Decode(bytes, flags, valuePtr)
			if errOut == nil {
				casOut = Cas(cas)
			}
		}
		signal <- true
	})
	if err != nil {
		return 0, err
	}

	timeoutTmr := gocbcore.AcquireTimer(b.opTimeout)
	select {
	case <-signal:
		gocbcore.ReleaseTimer(timeoutTmr, false)
		return
	case <-timeoutTmr.C:
		gocbcore.ReleaseTimer(timeoutTmr, true)
		if !op.Cancel() {
			<-signal
			return
		}
		return 0, ErrTimeout
	}
}

type hlpCasHandler func(ioCasCallback) (pendingOp, error)

func (b *Bucket) hlpCasExec(execFn hlpCasHandler) (casOut Cas, mtOut MutationToken, errOut error) {
	signal := make(chan bool, 1)
	op, err := execFn(func(cas gocbcore.Cas, mt gocbcore.MutationToken, err error) {
		errOut = err
		if errOut == nil {
			casOut = Cas(cas)
			mtOut = MutationToken{mt, b}
		}
		signal <- true
	})
	if err != nil {
		return 0, MutationToken{}, err
	}

	timeoutTmr := gocbcore.AcquireTimer(b.opTimeout)
	select {
	case <-signal:
		gocbcore.ReleaseTimer(timeoutTmr, false)
		return
	case <-timeoutTmr.C:
		gocbcore.ReleaseTimer(timeoutTmr, true)
		if !op.Cancel() {
			<-signal
			return
		}
		return 0, MutationToken{}, ErrTimeout
	}
}

type hlpCtrHandler func(ioCtrCallback) (pendingOp, error)

func (b *Bucket) hlpCtrExec(execFn hlpCtrHandler) (valOut uint64, casOut Cas, mtOut MutationToken, errOut error) {
	signal := make(chan bool, 1)
	op, err := execFn(func(value uint64, cas gocbcore.Cas, mt gocbcore.MutationToken, err error) {
		errOut = err
		if errOut == nil {
			valOut = value
			casOut = Cas(cas)
			mtOut = MutationToken{mt, b}
		}
		signal <- true
	})
	if err != nil {
		return 0, 0, MutationToken{}, err
	}

	timeoutTmr := gocbcore.AcquireTimer(b.opTimeout)
	select {
	case <-signal:
		gocbcore.ReleaseTimer(timeoutTmr, false)
		return
	case <-timeoutTmr.C:
		gocbcore.ReleaseTimer(timeoutTmr, true)
		if !op.Cancel() {
			<-signal
			return
		}
		return 0, 0, MutationToken{}, ErrTimeout
	}
}

func (b *Bucket) get(key string, valuePtr interface{}) (Cas, error) {
	return b.hlpGetExec(valuePtr, func(cb ioGetCallback) (pendingOp, error) {
		op, err := b.client.Get([]byte(key), gocbcore.GetCallback(cb))
		return op, err
	})
}

func (b *Bucket) getAndTouch(key string, expiry uint32, valuePtr interface{}) (Cas, error) {
	return b.hlpGetExec(valuePtr, func(cb ioGetCallback) (pendingOp, error) {
		op, err := b.client.GetAndTouch([]byte(key), expiry, gocbcore.GetCallback(cb))
		return op, err
	})
}

func (b *Bucket) getAndLock(key string, lockTime uint32, valuePtr interface{}) (Cas, error) {
	return b.hlpGetExec(valuePtr, func(cb ioGetCallback) (pendingOp, error) {
		op, err := b.client.GetAndLock([]byte(key), lockTime, gocbcore.GetCallback(cb))
		return op, err
	})
}

func (b *Bucket) unlock(key string, cas Cas) (Cas, MutationToken, error) {
	return b.hlpCasExec(func(cb ioCasCallback) (pendingOp, error) {
		op, err := b.client.Unlock([]byte(key), gocbcore.Cas(cas), gocbcore.UnlockCallback(cb))
		return op, err
	})
}

func (b *Bucket) getReplica(key string, valuePtr interface{}, replicaIdx int) (Cas, error) {
	return b.hlpGetExec(valuePtr, func(cb ioGetCallback) (pendingOp, error) {
		op, err := b.client.GetReplica([]byte(key), replicaIdx, gocbcore.GetCallback(cb))
		return op, err
	})
}

func (b *Bucket) touch(key string, cas Cas, expiry uint32) (Cas, MutationToken, error) {
	return b.hlpCasExec(func(cb ioCasCallback) (pendingOp, error) {
		op, err := b.client.Touch([]byte(key), gocbcore.Cas(cas), expiry, gocbcore.TouchCallback(cb))
		return op, err
	})
}

func (b *Bucket) remove(key string, cas Cas) (Cas, MutationToken, error) {
	return b.hlpCasExec(func(cb ioCasCallback) (pendingOp, error) {
		op, err := b.client.Remove([]byte(key), gocbcore.Cas(cas), gocbcore.RemoveCallback(cb))
		return op, err
	})
}

func (b *Bucket) upsert(key string, value interface{}, expiry uint32) (Cas, MutationToken, error) {
	bytes, flags, err := b.transcoder.Encode(value)
	if err != nil {
		return 0, MutationToken{}, err
	}

	return b.hlpCasExec(func(cb ioCasCallback) (pendingOp, error) {
		op, err := b.client.Set([]byte(key), bytes, flags, expiry, gocbcore.StoreCallback(cb))
		return op, err
	})
}

func (b *Bucket) insert(key string, value interface{}, expiry uint32) (Cas, MutationToken, error) {
	bytes, flags, err := b.transcoder.Encode(value)
	if err != nil {
		return 0, MutationToken{}, err
	}

	return b.hlpCasExec(func(cb ioCasCallback) (pendingOp, error) {
		op, err := b.client.Add([]byte(key), bytes, flags, expiry, gocbcore.StoreCallback(cb))
		return op, err
	})
}

func (b *Bucket) replace(key string, value interface{}, cas Cas, expiry uint32) (Cas, MutationToken, error) {
	bytes, flags, err := b.transcoder.Encode(value)
	if err != nil {
		return 0, MutationToken{}, err
	}

	return b.hlpCasExec(func(cb ioCasCallback) (pendingOp, error) {
		op, err := b.client.Replace([]byte(key), bytes, flags, gocbcore.Cas(cas), expiry, gocbcore.StoreCallback(cb))
		return op, err
	})
}

func (b *Bucket) append(key, value string) (Cas, MutationToken, error) {
	return b.hlpCasExec(func(cb ioCasCallback) (pendingOp, error) {
		op, err := b.client.Append([]byte(key), []byte(value), gocbcore.StoreCallback(cb))
		return op, err
	})
}

func (b *Bucket) prepend(key, value string) (Cas, MutationToken, error) {
	return b.hlpCasExec(func(cb ioCasCallback) (pendingOp, error) {
		op, err := b.client.Prepend([]byte(key), []byte(value), gocbcore.StoreCallback(cb))
		return op, err
	})
}

func (b *Bucket) counter(key string, delta, initial int64, expiry uint32) (uint64, Cas, MutationToken, error) {
	realInitial := uint64(0xFFFFFFFFFFFFFFFF)
	if initial >= 0 {
		realInitial = uint64(initial)
	}

	if delta > 0 {
		return b.hlpCtrExec(func(cb ioCtrCallback) (pendingOp, error) {
			op, err := b.client.Increment([]byte(key), uint64(delta), realInitial, expiry, gocbcore.CounterCallback(cb))
			return op, err
		})
	} else if delta < 0 {
		return b.hlpCtrExec(func(cb ioCtrCallback) (pendingOp, error) {
			op, err := b.client.Decrement([]byte(key), uint64(-delta), realInitial, expiry, gocbcore.CounterCallback(cb))
			return op, err
		})
	} else {
		return 0, 0, MutationToken{}, clientError{"Delta must be a non-zero value."}
	}
}
