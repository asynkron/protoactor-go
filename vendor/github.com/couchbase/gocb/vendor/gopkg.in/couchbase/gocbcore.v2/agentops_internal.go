package gocbcore

import (
	"encoding/binary"
)

// **INTERNAL**
// Stores a document along with setting some internal Couchbase meta-data.
func (c *Agent) SetMeta(key, value, extra []byte, options, flags, expiry uint32, cas, revseqno uint64, cb StoreCallback) (PendingOp, error) {
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

	extraBuf := make([]byte, 30+len(extra))
	binary.BigEndian.PutUint32(extraBuf[0:], flags)
	binary.BigEndian.PutUint32(extraBuf[4:], expiry)
	binary.BigEndian.PutUint64(extraBuf[8:], revseqno)
	binary.BigEndian.PutUint64(extraBuf[16:], cas)
	binary.BigEndian.PutUint32(extraBuf[24:], options)
	binary.BigEndian.PutUint16(extraBuf[28:], uint16(len(extra)))
	copy(extraBuf[30:], extra)
	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdSetMeta,
			Datatype: 0,
			Cas:      0,
			Extras:   extraBuf,
			Key:      key,
			Value:    value,
		},
		Callback: handler,
	}
	return c.dispatchOp(req)
}

// **INTERNAL**
// Deletes a document along with setting some internal Couchbase meta-data.
func (c *Agent) DeleteMeta(key, extra []byte, options, flags, expiry uint32, cas, revseqno uint64, cb RemoveCallback) (PendingOp, error) {
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

	extraBuf := make([]byte, 30+len(extra))
	binary.BigEndian.PutUint32(extraBuf[0:], flags)
	binary.BigEndian.PutUint32(extraBuf[4:], expiry)
	binary.BigEndian.PutUint64(extraBuf[8:], revseqno)
	binary.BigEndian.PutUint64(extraBuf[16:], cas)
	binary.BigEndian.PutUint32(extraBuf[24:], options)
	binary.BigEndian.PutUint16(extraBuf[28:], uint16(len(extra)))
	copy(extraBuf[30:], extra)
	req := &memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdDelMeta,
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
