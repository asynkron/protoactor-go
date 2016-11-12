package cluster

import (
	"bytes"
	"encoding/gob"
	"log"

	"github.com/hashicorp/memberlist"
)

type MemberlistGossiper struct {
	nodeName string
}

// NodeMeta is used to retrieve meta-data about the current node
// when broadcasting an alive message. It's length is limited to
// the given byte size. This metadata is available in the Node structure.
func (g *MemberlistGossiper) NodeMeta(limit int) []byte {
	log.Println("NodeMeta")
	return nil
}

// NotifyMsg is called when a user-data message is received.
// Care should be taken that this method does not block, since doing
// so would block the entire UDP packet receive loop. Additionally, the byte
// slice may be modified after the call returns, so it should be copied if needed.
func (g *MemberlistGossiper) NotifyMsg([]byte) {
	log.Println("NotifyMsg")
}

// GetBroadcasts is called when user data messages can be broadcast.
// It can return a list of buffers to send. Each buffer should assume an
// overhead as provided with a limit on the total byte size allowed.
// The total byte size of the resulting data to send must not exceed
// the limit.
func (g *MemberlistGossiper) GetBroadcasts(overhead, limit int) [][]byte {
	//log.Println("GetBroadcasts")
	return nil
}

// LocalState is used for a TCP Push/Pull. This is sent to
// the remote side in addition to the membership information. Any
// data can be sent here. See MergeRemoteState as well. The `join`
// boolean indicates this is for a join instead of a push/pull.
func (g *MemberlistGossiper) LocalState(join bool) []byte {

	log.Println("LocalState")
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(0)
	if err != nil {
		log.Println("err encoding counter:", err.Error())
		return nil
	}

	return buf.Bytes()
}

// MergeRemoteState is invoked after a TCP Push/Pull. This is the
// state received from the remote side and is the result of the
// remote side's LocalState call. The 'join'
// boolean indicates this is for a join instead of a push/pull.
func (g *MemberlistGossiper) MergeRemoteState(buf []byte, join bool) {
	log.Println("MergeRemoteState")
}

func NewMemberlistGossiper(nodeName string) memberlist.Delegate {
	return &MemberlistGossiper{nodeName}
}
