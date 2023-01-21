package actor

import (
	"sync/atomic"
	"unsafe"
)

/*
ensure the generated pid file contains the p *Process
TODO: make some sed command to inject this somehow

type PID struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Address   string `protobuf:"bytes,1,opt,name=Address,proto3" json:"Address,omitempty"`
	Id        string `protobuf:"bytes,2,opt,name=Id,proto3" json:"Id,omitempty"`
	RequestId uint32 `protobuf:"varint,3,opt,name=request_id,json=requestId,proto3" json:"request_id,omitempty"`

	//manually added
	p *Process
}
*/

//goland:noinspection GoReceiverNames
func (pid *PID) ref(actorSystem *ActorSystem) Process {
	p := (*Process)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&pid.p))))
	if p != nil {
		if l, ok := (*p).(*ActorProcess); ok && atomic.LoadInt32(&l.dead) == 1 {
			atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&pid.p)), nil)
		} else {
			return *p
		}
	}

	ref, exists := actorSystem.ProcessRegistry.Get(pid)
	if exists {
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&pid.p)), unsafe.Pointer(&ref))
	}

	return ref
}

// sendUserMessage sends a messages asynchronously to the PID.
//
//goland:noinspection GoReceiverNames
func (pid *PID) sendUserMessage(actorSystem *ActorSystem, message interface{}) {
	pid.ref(actorSystem).SendUserMessage(pid, message)
}

//goland:noinspection GoReceiverNames.
func (pid *PID) sendSystemMessage(actorSystem *ActorSystem, message interface{}) {
	pid.ref(actorSystem).SendSystemMessage(pid, message)
}

//goland:noinspection GoReceiverNames.
func (pid *PID) Equal(other *PID) bool {
	if pid != nil && other == nil {
		return false
	}

	return pid.Id == other.Id && pid.Address == other.Address && pid.RequestId == other.RequestId
}

// NewPID returns a new instance of the PID struct.
func NewPID(address, id string) *PID {
	return &PID{
		Address: address,
		Id:      id,
	}
}
