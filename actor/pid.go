package actor

import (
	"sync/atomic"
	"time"
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

// StopFuture will stop actor immediately regardless of existing user messages in mailbox, and return its future.
//
// Deprecated: Use Context.StopFuture instead
func (pid *PID) StopFuture(actorSystem *ActorSystem) *Future {
	future := NewFuture(actorSystem, 10*time.Second)

	pid.sendSystemMessage(actorSystem, &Watch{Watcher: future.pid})
	actorSystem.Root.Stop(pid)

	return future
}

// GracefulStop will stop actor immediately regardless of existing user messages in mailbox.
//
// Deprecated: Use Context.StopFuture(pid).Wait() instead
func (pid *PID) GracefulStop(actorSystem *ActorSystem) {
	pid.StopFuture(actorSystem).Wait()
}

// Stop will stop actor immediately regardless of existing user messages in mailbox.
//
// Deprecated: Use Context.Stop instead
func (pid *PID) Stop(actorSystem *ActorSystem) {
	pid.ref(actorSystem).Stop(pid)
}

// PoisonFuture will tell actor to stop after processing current user messages in mailbox, and return its future.
//
// Deprecated: Use Context.PoisonFuture instead
func (pid *PID) PoisonFuture(actorSystem *ActorSystem) *Future {
	future := NewFuture(actorSystem, 10*time.Second)

	pid.sendSystemMessage(actorSystem, &Watch{Watcher: future.pid})
	pid.Poison(actorSystem)

	return future
}

// GracefulPoison will tell and wait actor to stop after processing current user messages in mailbox.
//
// Deprecated: Use Context.PoisonFuture(pid).Wait() instead
func (pid *PID) GracefulPoison(actorSystem *ActorSystem) {
	pid.PoisonFuture(actorSystem).Wait()
}

// Poison will tell actor to stop after processing current user messages in mailbox.
//
// Deprecated: Use Context.Poison instead
func (pid *PID) Poison(actorSystem *ActorSystem) {
	pid.sendUserMessage(actorSystem, poisonPillMessage)
}
