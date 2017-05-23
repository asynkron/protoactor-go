package gocbcore

import (
	"encoding/binary"
)

// Retrieves the current CAS and persistence state for a document.
func (c *Agent) Observe(key []byte, replicaIdx int, cb ObserveCallback) (PendingOp, error) {
	// TODO(mnunberg): Use bktType when implemented
	if c.numVbuckets == 0 {
		return nil, ErrNotSupported
	}

	handler := func(resp *memdResponse, _ *memdRequest, err error) {
		if err != nil {
			cb(0, 0, err)
			return
		}

		if len(resp.Value) < 4 {
			cb(0, 0, ErrProtocol)
			return
		}
		keyLen := int(binary.BigEndian.Uint16(resp.Value[2:]))

		if len(resp.Value) != 2+2+keyLen+1+8 {
			cb(0, 0, ErrProtocol)
			return
		}
		keyState := KeyState(resp.Value[2+2+keyLen])
		cas := binary.BigEndian.Uint64(resp.Value[2+2+keyLen+1:])

		cb(keyState, Cas(cas), nil)
	}

	vbId := c.KeyToVbucket(key)

	valueBuf := make([]byte, 2+2+len(key))
	binary.BigEndian.PutUint16(valueBuf[0:], vbId)
	binary.BigEndian.PutUint16(valueBuf[2:], uint16(len(key)))
	copy(valueBuf[4:], key)

	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdObserve,
			Datatype: 0,
			Cas:      0,
			Extras:   nil,
			Key:      nil,
			Value:    valueBuf,
			Vbucket:  vbId,
		},
		ReplicaIdx: replicaIdx,
		Callback:   handler,
	}
	return c.dispatchOp(req)
}

// Retrieves the persistence state sequence numbers for a particular VBucket.
func (c *Agent) ObserveSeqNo(key []byte, vbUuid VbUuid, replicaIdx int, cb ObserveSeqNoCallback) (PendingOp, error) {
	// TODO(mnunberg): Use bktType when implemented
	if c.numVbuckets == 0 {
		return nil, ErrNotSupported
	}

	handler := func(resp *memdResponse, _ *memdRequest, err error) {
		if err != nil {
			cb(0, 0, err)
			return
		}

		if len(resp.Value) < 1 {
			cb(0, 0, ErrProtocol)
			return
		}

		formatType := resp.Value[0]
		if formatType == 0 {
			// Normal
			if len(resp.Value) < 27 {
				cb(0, 0, ErrProtocol)
				return
			}

			//vbId := binary.BigEndian.Uint16(resp.Value[1:])
			//vbUuid := binary.BigEndian.Uint64(resp.Value[3:])
			persistSeqNo := binary.BigEndian.Uint64(resp.Value[11:])
			currentSeqNo := binary.BigEndian.Uint64(resp.Value[19:])

			cb(SeqNo(currentSeqNo), SeqNo(persistSeqNo), nil)
			return
		} else if formatType == 1 {
			// Hard Failover
			if len(resp.Value) < 43 {
				cb(0, 0, ErrProtocol)
				return
			}

			//vbId := binary.BigEndian.Uint16(resp.Value[1:])
			//newVbUuid := binary.BigEndian.Uint64(resp.Value[3:])
			//persistSeqNo := binary.BigEndian.Uint64(resp.Value[11:])
			//currentSeqNo := binary.BigEndian.Uint64(resp.Value[19:])
			//vbUuid := binary.BigEndian.Uint64(resp.Value[27:])
			lastSeqNo := binary.BigEndian.Uint64(resp.Value[35:])

			cb(SeqNo(lastSeqNo), SeqNo(lastSeqNo), nil)
			return
		} else {
			cb(0, 0, ErrProtocol)
			return
		}
	}

	vbId := c.KeyToVbucket(key)

	valueBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(valueBuf[0:], uint64(vbUuid))

	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdObserveSeqNo,
			Datatype: 0,
			Cas:      0,
			Extras:   nil,
			Key:      nil,
			Value:    valueBuf,
			Vbucket:  vbId,
		},
		ReplicaIdx: replicaIdx,
		Callback:   handler,
	}
	return c.dispatchOp(req)
}
