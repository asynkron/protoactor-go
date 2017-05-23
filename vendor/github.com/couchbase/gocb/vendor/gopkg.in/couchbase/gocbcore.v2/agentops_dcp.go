package gocbcore

import (
	"encoding/binary"
)

// Represents the state of a particular cluster snapshot.
type SnapshotState uint32

// Returns whether this snapshot is available in memory.
func (s SnapshotState) HasInMemory() bool {
	return uint32(s)&1 != 0
}

// Returns whether this snapshot is available on disk.
func (s SnapshotState) HasOnDisk() bool {
	return uint32(s)&2 != 0
}

// Represents a single entry in the server failover log.
type FailoverEntry struct {
	VbUuid VbUuid
	SeqNo  SeqNo
}

type StreamObserver interface {
	SnapshotMarker(startSeqNo, endSeqNo uint64, vbId uint16, snapshotType SnapshotState)
	Mutation(seqNo, revNo uint64, flags, expiry, lockTime uint32, cas uint64, datatype uint8, vbId uint16, key, value []byte)
	Deletion(seqNo, revNo, cas uint64, vbId uint16, key []byte)
	Expiration(seqNo, revNo, cas uint64, vbId uint16, key []byte)
	End(vbId uint16, err error)
}

type OpenStreamCallback func([]FailoverEntry, error)
type CloseStreamCallback func(error)
type GetFailoverLogCallback func([]FailoverEntry, error)
type GetVBucketSeqnosCallback func(uint16, SeqNo, error)

// **INTERNAL**
// Opens a DCP stream for a particular VBucket.
func (c *Agent) OpenStream(vbId uint16, vbUuid VbUuid, startSeqNo, endSeqNo, snapStartSeqNo, snapEndSeqNo SeqNo, evtHandler StreamObserver, cb OpenStreamCallback) (PendingOp, error) {
	var req *memdQRequest
	handler := func(resp *memdResponse, _ *memdRequest, err error) {
		if resp.Magic == ResMagic {
			// This is the response to the open stream request.
			if err != nil {
				// All client errors are handled by the StreamObserver
				cb(nil, err)
				return
			}

			numEntries := len(resp.Value) / 16
			entries := make([]FailoverEntry, numEntries)
			for i := 0; i < numEntries; i++ {
				entries[i] = FailoverEntry{
					VbUuid: VbUuid(binary.BigEndian.Uint64(resp.Value[i*16+0:])),
					SeqNo:  SeqNo(binary.BigEndian.Uint64(resp.Value[i*16+8:])),
				}
			}

			cb(entries, nil)
			return
		}

		if err != nil {
			evtHandler.End(vbId, err)
			return
		}

		// This is one of the stream events
		switch resp.Opcode {
		case CmdDcpSnapshotMarker:
			vbId := uint16(resp.Status)
			newStartSeqNo := binary.BigEndian.Uint64(resp.Extras[0:])
			newEndSeqNo := binary.BigEndian.Uint64(resp.Extras[8:])
			snapshotType := binary.BigEndian.Uint32(resp.Extras[16:])
			evtHandler.SnapshotMarker(newStartSeqNo, newEndSeqNo, vbId, SnapshotState(snapshotType))
		case CmdDcpMutation:
			vbId := uint16(resp.Status)
			seqNo := binary.BigEndian.Uint64(resp.Extras[0:])
			revNo := binary.BigEndian.Uint64(resp.Extras[8:])
			flags := binary.BigEndian.Uint32(resp.Extras[16:])
			expiry := binary.BigEndian.Uint32(resp.Extras[20:])
			lockTime := binary.BigEndian.Uint32(resp.Extras[24:])
			evtHandler.Mutation(seqNo, revNo, flags, expiry, lockTime, resp.Cas, resp.Datatype, vbId, resp.Key, resp.Value)
		case CmdDcpDeletion:
			vbId := uint16(resp.Status)
			seqNo := binary.BigEndian.Uint64(resp.Extras[0:])
			revNo := binary.BigEndian.Uint64(resp.Extras[8:])
			evtHandler.Deletion(seqNo, revNo, resp.Cas, vbId, resp.Key)
		case CmdDcpExpiration:
			vbId := uint16(resp.Status)
			seqNo := binary.BigEndian.Uint64(resp.Extras[0:])
			revNo := binary.BigEndian.Uint64(resp.Extras[8:])
			evtHandler.Expiration(seqNo, revNo, resp.Cas, vbId, resp.Key)
		case CmdDcpStreamEnd:
			vbId := uint16(resp.Status)
			code := StreamEndStatus(binary.BigEndian.Uint32(resp.Extras[0:]))
			evtHandler.End(vbId, getStreamEndError(code))
			req.Cancel()
		}
	}

	extraBuf := make([]byte, 48)
	binary.BigEndian.PutUint32(extraBuf[0:], 0)
	binary.BigEndian.PutUint32(extraBuf[4:], 0)
	binary.BigEndian.PutUint64(extraBuf[8:], uint64(startSeqNo))
	binary.BigEndian.PutUint64(extraBuf[16:], uint64(endSeqNo))
	binary.BigEndian.PutUint64(extraBuf[24:], uint64(vbUuid))
	binary.BigEndian.PutUint64(extraBuf[32:], uint64(snapStartSeqNo))
	binary.BigEndian.PutUint64(extraBuf[40:], uint64(snapEndSeqNo))

	req = &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdDcpStreamReq,
			Datatype: 0,
			Cas:      0,
			Extras:   extraBuf,
			Key:      nil,
			Value:    nil,
			Vbucket:  vbId,
		},
		Callback:   handler,
		ReplicaIdx: 0,
		Persistent: true,
	}
	return c.dispatchOp(req)
}

// **INTERNAL**
// Shuts down an open stream for the specified VBucket.
func (c *Agent) CloseStream(vbId uint16, cb CloseStreamCallback) (PendingOp, error) {
	handler := func(resp *memdResponse, _ *memdRequest, err error) {
		cb(err)
	}

	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdDcpCloseStream,
			Datatype: 0,
			Cas:      0,
			Extras:   nil,
			Key:      nil,
			Value:    nil,
			Vbucket:  vbId,
		},
		Callback:   handler,
		ReplicaIdx: 0,
		Persistent: false,
	}
	return c.dispatchOp(req)
}

// **INTERNAL**
// Retrieves the failover log for a particular VBucket.  This is used
// to resume an interrupted stream after a node failover has occured.
func (c *Agent) GetFailoverLog(vbId uint16, cb GetFailoverLogCallback) (PendingOp, error) {
	handler := func(resp *memdResponse, _ *memdRequest, err error) {
		if err != nil {
			cb(nil, err)
			return
		}

		numEntries := len(resp.Value) / 16
		entries := make([]FailoverEntry, numEntries)
		for i := 0; i < numEntries; i++ {
			entries[i] = FailoverEntry{
				VbUuid: VbUuid(binary.BigEndian.Uint64(resp.Value[i*16+0:])),
				SeqNo:  SeqNo(binary.BigEndian.Uint64(resp.Value[i*16+8:])),
			}
		}
		cb(entries, nil)
	}

	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdDcpGetFailoverLog,
			Datatype: 0,
			Cas:      0,
			Extras:   nil,
			Key:      nil,
			Value:    nil,
			Vbucket:  vbId,
		},
		Callback:   handler,
		ReplicaIdx: 0,
		Persistent: false,
	}
	return c.dispatchOp(req)
}

// **INTERNAL**
// Returns the last checkpoint for a particular VBucket.  This is useful
// for starting a DCP stream from wherever the server currently is.
func (c *Agent) GetVbucketSeqnos(serverIdx int, cb GetVBucketSeqnosCallback) (PendingOp, error) {
	handler := func(resp *memdResponse, _ *memdRequest, err error) {
		if err != nil {
			cb(0, 0, err)
			return
		}

		vbs := len(resp.Value) / 10
		for i := 0; i < vbs; i++ {
			vbid := binary.BigEndian.Uint16(resp.Value[i*10:])
			seqNo := SeqNo(binary.BigEndian.Uint64(resp.Value[i*10+2:]))
			cb(vbid, seqNo, nil)
		}
	}

	extraBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(extraBuf[0:], uint32(VBucketStateActive))

	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdGetAllVBSeqnos,
			Datatype: 0,
			Cas:      0,
			Extras:   extraBuf,
			Key:      nil,
			Value:    nil,
			Vbucket:  0,
		},
		Callback:   handler,
		ReplicaIdx: -serverIdx,
		Persistent: false,
	}

	return c.dispatchOp(req)
}
