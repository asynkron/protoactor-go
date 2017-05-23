package gocbcore

import (
	"encoding/binary"
	"sync/atomic"
)

// Retrieves a document.
func (c *Agent) Get(key []byte, cb GetCallback) (PendingOp, error) {
	handler := func(resp *memdResponse, req *memdRequest, err error) {
		if err != nil {
			cb(nil, 0, 0, err)
			return
		}
		flags := binary.BigEndian.Uint32(resp.Extras[0:])
		cb(resp.Value, flags, Cas(resp.Cas), nil)
	}
	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdGet,
			Datatype: 0,
			Cas:      0,
			Extras:   nil,
			Key:      key,
			Value:    nil,
		},
		Callback: handler,
	}
	return c.dispatchOp(req)
}

// Retrieves a document and updates its expiry.
func (c *Agent) GetAndTouch(key []byte, expiry uint32, cb GetCallback) (PendingOp, error) {
	handler := func(resp *memdResponse, req *memdRequest, err error) {
		if err != nil {
			cb(nil, 0, 0, err)
			return
		}
		flags := binary.BigEndian.Uint32(resp.Extras[0:])
		cb(resp.Value, flags, Cas(resp.Cas), nil)
	}

	extraBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(extraBuf[0:], expiry)

	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdGAT,
			Datatype: 0,
			Cas:      0,
			Extras:   extraBuf,
			Key:      key,
			Value:    nil,
		},
		Callback: handler,
	}
	return c.dispatchOp(req)
}

// Retrieves a document and locks it.
func (c *Agent) GetAndLock(key []byte, lockTime uint32, cb GetCallback) (PendingOp, error) {
	handler := func(resp *memdResponse, req *memdRequest, err error) {
		if err != nil {
			cb(nil, 0, 0, err)
			return
		}
		flags := binary.BigEndian.Uint32(resp.Extras[0:])
		cb(resp.Value, flags, Cas(resp.Cas), nil)
	}

	extraBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(extraBuf[0:], lockTime)

	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdGetLocked,
			Datatype: 0,
			Cas:      0,
			Extras:   extraBuf,
			Key:      key,
			Value:    nil,
		},
		Callback: handler,
	}
	return c.dispatchOp(req)
}

func (c *Agent) getOneReplica(key []byte, replicaIdx int, cb GetCallback) (PendingOp, error) {
	if replicaIdx <= 0 {
		panic("Replica number must be greater than 0")
	}

	handler := func(resp *memdResponse, req *memdRequest, err error) {
		if err != nil {
			cb(nil, 0, 0, err)
			return
		}
		flags := binary.BigEndian.Uint32(resp.Extras[0:])
		cb(resp.Value, flags, Cas(resp.Cas), nil)
	}

	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdGetReplica,
			Datatype: 0,
			Cas:      0,
			Extras:   nil,
			Key:      key,
			Value:    nil,
		},
		Callback:   handler,
		ReplicaIdx: replicaIdx,
	}
	return c.dispatchOp(req)
}

func (c *Agent) getAnyReplica(key []byte, cb GetCallback) (PendingOp, error) {
	opRes := &multiPendingOp{}

	var cbCalled uint32
	handler := func(value []byte, flags uint32, cas Cas, err error) {
		if atomic.CompareAndSwapUint32(&cbCalled, 0, 1) {
			// Cancel all other commands if possible.
			opRes.Cancel()
			// Dispatch Callback
			cb(value, flags, cas, err)
		}
	}

	// Dispatch a getReplica for each replica server
	numReplicas := c.NumReplicas()
	for repIdx := 1; repIdx <= numReplicas; repIdx++ {
		op, err := c.getOneReplica(key, repIdx, handler)
		if err == nil {
			opRes.ops = append(opRes.ops, op)
		}
	}

	// If we have no pending ops, no requests were successful
	if len(opRes.ops) == 0 {
		return nil, ErrNoReplicas
	}

	return opRes, nil
}

// Retrieves a document from a replica server.
func (c *Agent) GetReplica(key []byte, replicaIdx int, cb GetCallback) (PendingOp, error) {
	if replicaIdx > 0 {
		return c.getOneReplica(key, replicaIdx, cb)
	} else if replicaIdx == 0 {
		return c.getAnyReplica(key, cb)
	} else {
		panic("Replica number must not be less than 0.")
	}
}

// Touches a document, updating its expiry.
func (c *Agent) Touch(key []byte, cas Cas, expiry uint32, cb TouchCallback) (PendingOp, error) {
	handler := func(resp *memdResponse, req *memdRequest, err error) {
		if err != nil {
			cb(0, MutationToken{}, err)
			return
		}

		mutToken := MutationToken{}
		if len(resp.Extras) >= 16 {
			mutToken.VbId = c.KeyToVbucket(key)
			mutToken.VbUuid = VbUuid(binary.BigEndian.Uint64(resp.Extras[0:]))
			mutToken.SeqNo = SeqNo(binary.BigEndian.Uint64(resp.Extras[8:]))
		}

		cb(Cas(resp.Cas), mutToken, nil)
	}

	extraBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(extraBuf[0:], expiry)

	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdTouch,
			Datatype: 0,
			Cas:      uint64(cas),
			Extras:   extraBuf,
			Key:      key,
			Value:    nil,
		},
		Callback: handler,
	}
	return c.dispatchOp(req)
}

// Unlocks a locked document.
func (c *Agent) Unlock(key []byte, cas Cas, cb UnlockCallback) (PendingOp, error) {
	handler := func(resp *memdResponse, req *memdRequest, err error) {
		if err != nil {
			cb(0, MutationToken{}, err)
			return
		}

		mutToken := MutationToken{}
		if len(resp.Extras) >= 16 {
			mutToken.VbId = req.Vbucket
			mutToken.VbUuid = VbUuid(binary.BigEndian.Uint64(resp.Extras[0:]))
			mutToken.SeqNo = SeqNo(binary.BigEndian.Uint64(resp.Extras[8:]))
		}

		cb(Cas(resp.Cas), mutToken, nil)
	}

	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdUnlockKey,
			Datatype: 0,
			Cas:      uint64(cas),
			Extras:   nil,
			Key:      key,
			Value:    nil,
		},
		Callback: handler,
	}
	return c.dispatchOp(req)
}

// Removes a document.
func (c *Agent) Remove(key []byte, cas Cas, cb RemoveCallback) (PendingOp, error) {
	handler := func(resp *memdResponse, req *memdRequest, err error) {
		if err != nil {
			cb(0, MutationToken{}, err)
			return
		}

		mutToken := MutationToken{}
		if len(resp.Extras) >= 16 {
			mutToken.VbId = req.Vbucket
			mutToken.VbUuid = VbUuid(binary.BigEndian.Uint64(resp.Extras[0:]))
			mutToken.SeqNo = SeqNo(binary.BigEndian.Uint64(resp.Extras[8:]))
		}

		cb(Cas(resp.Cas), mutToken, nil)
	}

	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdDelete,
			Datatype: 0,
			Cas:      uint64(cas),
			Extras:   nil,
			Key:      key,
			Value:    nil,
		},
		Callback: handler,
	}
	return c.dispatchOp(req)
}

func (c *Agent) store(opcode CommandCode, key, value []byte, flags uint32, cas Cas, expiry uint32, cb StoreCallback) (PendingOp, error) {
	handler := func(resp *memdResponse, req *memdRequest, err error) {
		if err != nil {
			cb(0, MutationToken{}, err)
			return
		}

		mutToken := MutationToken{}
		if len(resp.Extras) >= 16 {
			mutToken.VbId = req.Vbucket
			mutToken.VbUuid = VbUuid(binary.BigEndian.Uint64(resp.Extras[0:]))
			mutToken.SeqNo = SeqNo(binary.BigEndian.Uint64(resp.Extras[8:]))
		}

		cb(Cas(resp.Cas), mutToken, nil)
	}

	extraBuf := make([]byte, 8)
	binary.BigEndian.PutUint32(extraBuf[0:], flags)
	binary.BigEndian.PutUint32(extraBuf[4:], expiry)
	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   opcode,
			Datatype: 0,
			Cas:      uint64(cas),
			Extras:   extraBuf,
			Key:      key,
			Value:    value,
		},
		Callback: handler,
	}
	return c.dispatchOp(req)
}

// Stores a document as long as it does not already exist.
func (c *Agent) Add(key, value []byte, flags uint32, expiry uint32, cb StoreCallback) (PendingOp, error) {
	return c.store(CmdAdd, key, value, flags, 0, expiry, cb)
}

// Stores a document.
func (c *Agent) Set(key, value []byte, flags uint32, expiry uint32, cb StoreCallback) (PendingOp, error) {
	return c.store(CmdSet, key, value, flags, 0, expiry, cb)
}

// Replaces the value of a Couchbase document with another value.
func (c *Agent) Replace(key, value []byte, flags uint32, cas Cas, expiry uint32, cb StoreCallback) (PendingOp, error) {
	return c.store(CmdReplace, key, value, flags, cas, expiry, cb)
}

// Performs an adjoin operation.
func (c *Agent) adjoin(opcode CommandCode, key, value []byte, cb StoreCallback) (PendingOp, error) {
	handler := func(resp *memdResponse, req *memdRequest, err error) {
		if err != nil {
			cb(0, MutationToken{}, err)
			return
		}

		mutToken := MutationToken{}
		if len(resp.Extras) >= 16 {
			mutToken.VbId = req.Vbucket
			mutToken.VbUuid = VbUuid(binary.BigEndian.Uint64(resp.Extras[0:]))
			mutToken.SeqNo = SeqNo(binary.BigEndian.Uint64(resp.Extras[8:]))
		}

		cb(Cas(resp.Cas), mutToken, nil)
	}

	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   opcode,
			Datatype: 0,
			Cas:      0,
			Extras:   nil,
			Key:      key,
			Value:    value,
		},
		Callback: handler,
	}
	return c.dispatchOp(req)
}

// Appends some bytes to a document.
func (c *Agent) Append(key, value []byte, cb StoreCallback) (PendingOp, error) {
	return c.adjoin(CmdAppend, key, value, cb)
}

// Prepends some bytes to a document.
func (c *Agent) Prepend(key, value []byte, cb StoreCallback) (PendingOp, error) {
	return c.adjoin(CmdPrepend, key, value, cb)
}

// Performs a counter operation.
func (c *Agent) counter(opcode CommandCode, key []byte, delta, initial uint64, expiry uint32, cb CounterCallback) (PendingOp, error) {
	handler := func(resp *memdResponse, req *memdRequest, err error) {
		if err != nil {
			cb(0, 0, MutationToken{}, err)
			return
		}

		if len(resp.Value) != 8 {
			cb(0, 0, MutationToken{}, ErrProtocol)
			return
		}
		intVal := binary.BigEndian.Uint64(resp.Value)

		mutToken := MutationToken{}
		if len(resp.Extras) >= 16 {
			mutToken.VbId = req.Vbucket
			mutToken.VbUuid = VbUuid(binary.BigEndian.Uint64(resp.Extras[0:]))
			mutToken.SeqNo = SeqNo(binary.BigEndian.Uint64(resp.Extras[8:]))
		}

		cb(intVal, Cas(resp.Cas), mutToken, nil)
	}

	// You cannot have an expiry when you do not want to create the document.
	if initial == uint64(0xFFFFFFFFFFFFFFFF) && expiry != 0 {
		return nil, ErrInvalidArgs
	}

	extraBuf := make([]byte, 20)
	binary.BigEndian.PutUint64(extraBuf[0:], delta)
	if initial != uint64(0xFFFFFFFFFFFFFFFF) {
		binary.BigEndian.PutUint64(extraBuf[8:], initial)
		binary.BigEndian.PutUint32(extraBuf[16:], expiry)
	} else {
		binary.BigEndian.PutUint64(extraBuf[8:], 0x0000000000000000)
		binary.BigEndian.PutUint32(extraBuf[16:], 0xFFFFFFFF)
	}

	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   opcode,
			Datatype: 0,
			Cas:      0,
			Extras:   extraBuf,
			Key:      key,
			Value:    nil,
		},
		Callback: handler,
	}
	return c.dispatchOp(req)
}

// Increments the unsigned integer value in a document.
func (c *Agent) Increment(key []byte, delta, initial uint64, expiry uint32, cb CounterCallback) (PendingOp, error) {
	return c.counter(CmdIncrement, key, delta, initial, expiry, cb)
}

// Decrements the unsigned integer value in a document.
func (c *Agent) Decrement(key []byte, delta, initial uint64, expiry uint32, cb CounterCallback) (PendingOp, error) {
	return c.counter(CmdDecrement, key, delta, initial, expiry, cb)
}

// *VOLATILE*
// Returns the key and value of a random document stored within Couchbase Server.
func (c *Agent) GetRandom(cb GetRandomCallback) (PendingOp, error) {
	handler := func(resp *memdResponse, _ *memdRequest, err error) {
		if err != nil {
			cb(nil, nil, 0, 0, err)
			return
		}
		flags := binary.BigEndian.Uint32(resp.Extras[0:])
		cb(resp.Key, resp.Value, flags, Cas(resp.Cas), nil)
	}
	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdGetRandom,
			Datatype: 0,
			Cas:      0,
			Extras:   nil,
			Key:      nil,
			Value:    nil,
		},
		Callback: handler,
	}
	return c.dispatchOp(req)
}

func (c *Agent) Stats(key string, callback ServerStatsCallback) (PendingOp, error) {
	config := c.routingInfo.get()
	allOk := true
	// Iterate over each of the configs

	op := new(struct {
		multiPendingOp
		remaining int32
	})
	op.remaining = int32(len(config.servers))

	stats := make(map[string]SingleServerStats)

	defer func() {
		if !allOk {
			op.Cancel()
		}
	}()

	for index, server := range config.servers {
		var req *memdQRequest
		serverName := server.address

		handler := func(resp *memdResponse, _ *memdRequest, err error) {
			// No stat key!
			curStats, ok := stats[serverName]

			if !ok {
				stats[serverName] = SingleServerStats{
					Stats: make(map[string]string),
				}
				curStats = stats[serverName]
			}
			if err != nil {
				if curStats.Error == nil {
					curStats.Error = err
				} else {
					logDebugf("Got additional error for stats: %s: %v", serverName, err)
				}
			}

			if len(resp.Key) == 0 {
				// No more request for server!
				req.Cancel()

				remaining := atomic.AddInt32(&op.remaining, -1)
				if remaining == 0 {
					callback(stats)
				}
			} else {
				curStats.Stats[string(resp.Key)] = string(resp.Value)
			}
		}

		// Send the request
		req = &memdQRequest{
			memdRequest: memdRequest{
				Magic:    ReqMagic,
				Opcode:   CmdStat,
				Datatype: 0,
				Cas:      0,
				Key:      []byte(key),
				Value:    nil,
			},
			Persistent: true,
			ReplicaIdx: (-1) + (-index),
			Callback:   handler,
		}

		curOp, err := c.dispatchOp(req)
		if err != nil {
			return nil, err
		}
		op.ops = append(op.ops, curOp)
	}
	allOk = true
	return op, nil
}
