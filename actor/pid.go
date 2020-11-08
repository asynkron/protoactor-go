package actor

import (
	"sync/atomic"
	"unsafe"
)

type PID struct {
	Address string `protobuf:"bytes,1,opt,name=Address,proto3" json:"Address,omitempty"`
	Id      string `protobuf:"bytes,2,opt,name=Id,proto3" json:"Id,omitempty"`

	p *Process
}

/*
func (m *PID) MarshalJSONPB(*jsonpb.Marshaler) ([]byte, error) {
	str := fmt.Sprintf("{\"Address\":\"%v\", \"Id\":\"%v\"}", m.Address, m.Id)
	return []byte(str), nil
}*/

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

// sendUserMessage sends a messages asynchronously to the PID
func (pid *PID) sendUserMessage(actorSystem *ActorSystem, message interface{}) {
	pid.ref(actorSystem).SendUserMessage(pid, message)
}

func (pid *PID) sendSystemMessage(actorSystem *ActorSystem, message interface{}) {
	pid.ref(actorSystem).SendSystemMessage(pid, message)
}

func (pid *PID) String() string {
	if pid == nil {
		return "nil"
	}
	return pid.Address + "/" + pid.Id
}

// NewPID returns a new instance of the PID struct
func NewPID(address, id string) *PID {
	return &PID{
		Address: address,
		Id:      id,
	}
}
