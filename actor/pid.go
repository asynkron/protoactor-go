package actor

import (
	//	"fmt"
	//	"github.com/gogo/protobuf/jsonpb"
	"strings"
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

func (pid *PID) ref() Process {
	p := (*Process)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&pid.p))))
	if p != nil {
		if l, ok := (*p).(*ActorProcess); ok && atomic.LoadInt32(&l.dead) == 1 {
			atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&pid.p)), nil)
		} else {
			return *p
		}
	}

	ref, exists := ProcessRegistry.Get(pid)
	if exists {
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&pid.p)), unsafe.Pointer(&ref))
	}

	return ref
}

// sendUserMessage sends a messages asynchronously to the PID
func (pid *PID) sendUserMessage(message interface{}) {
	pid.ref().SendUserMessage(pid, message)
}

func (pid *PID) sendSystemMessage(message interface{}) {
	pid.ref().SendSystemMessage(pid, message)
}

func (pid *PID) key() string {
	if pid.Address == ProcessRegistry.Address {
		return pid.Id
	}
	return pid.Address + "#" + pid.Id
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

// NewLocalPID returns a new instance of the PID struct with the address preset
func NewLocalPID(id string) *PID {
	return &PID{
		Address: ProcessRegistry.Address,
		Id:      id,
	}
}

func pidFromKey(key string, p *PID) {
	i := strings.IndexByte(key, '#')
	if i == -1 {
		p.Address = ProcessRegistry.Address
		p.Id = key
	} else {
		p.Address = key[:i]
		p.Id = key[i+1:]
	}
}

// Deprecated: Use Context.Send instead
func (pid *PID) Tell(message interface{}) {
	ctx := EmptyRootContext
	ctx.Send(pid, message)
}

// Deprecated: Use Context.Request or Context.RequestWithCustomSender instead
func (pid *PID) Request(message interface{}, respondTo *PID) {
	ctx := EmptyRootContext
	ctx.RequestWithCustomSender(pid, message, respondTo)
}

// Deprecated: Use Context.RequestFuture instead
func (pid *PID) RequestFuture(message interface{}, timeout time.Duration) *Future {
	ctx := EmptyRootContext
	return ctx.RequestFuture(pid, message, timeout)
}

// StopFuture will stop actor immediately regardless of existing user messages in mailbox, and return its future.
//
// Deprecated: Use Context.StopFuture instead
func (pid *PID) StopFuture() *Future {
	future := NewFuture(10 * time.Second)

	pid.sendSystemMessage(&Watch{Watcher: future.pid})
	pid.Stop()

	return future
}

// GracefulStop will stop actor immediately regardless of existing user messages in mailbox.
//
// Deprecated: Use Context.StopFuture(pid).Wait() instead
func (pid *PID) GracefulStop() {
	pid.StopFuture().Wait()
}

// Stop will stop actor immediately regardless of existing user messages in mailbox.
//
// Deprecated: Use Context.Stop instead
func (pid *PID) Stop() {
	pid.ref().Stop(pid)
}

// PoisonFuture will tell actor to stop after processing current user messages in mailbox, and return its future.
//
// Deprecated: Use Context.PoisonFuture instead
func (pid *PID) PoisonFuture() *Future {
	future := NewFuture(10 * time.Second)

	pid.sendSystemMessage(&Watch{Watcher: future.pid})
	pid.Poison()

	return future
}

// GracefulPoison will tell and wait actor to stop after processing current user messages in mailbox.
//
// Deprecated: Use Context.PoisonFuture(pid).Wait() instead
func (pid *PID) GracefulPoison() {
	pid.PoisonFuture().Wait()
}

// Poison will tell actor to stop after processing current user messages in mailbox.
//
// Deprecated: Use Context.Poison instead
func (pid *PID) Poison() {
	pid.sendUserMessage(&PoisonPill{})
}
