package actor

import (
	"sync/atomic"

	cmap "github.com/orcaman/concurrent-map"
)

type ProcessRegistryValue struct {
	SequenceID     uint64
	Address        string
	LocalPIDs      cmap.ConcurrentMap
	RemoteHandlers []AddressResolver
}

var (
	localAddress = "nonhost"
)

// ProcessRegistry is a registry of all active processes.
//
// NOTE: This should only be used for advanced scenarios
var ProcessRegistry = &ProcessRegistryValue{
	Address:   localAddress,
	LocalPIDs: cmap.New(),
}

// An AddressResolver is used to resolve remote actors
type AddressResolver func(*PID) (Process, bool)

func (pr *ProcessRegistryValue) RegisterAddressResolver(handler AddressResolver) {
	pr.RemoteHandlers = append(pr.RemoteHandlers, handler)
}

const (
	digits = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ~+"
)

func uint64ToId(u uint64) string {
	var buf [13]byte
	i := 13
	// base is power of 2: use shifts and masks instead of / and %
	for u >= 64 {
		i--
		buf[i] = digits[uintptr(u)&0x3f]
		u >>= 6
	}
	// u < base
	i--
	buf[i] = digits[uintptr(u)]
	i--
	buf[i] = '$'

	return string(buf[i:])
}

func (pr *ProcessRegistryValue) NextId() string {
	counter := atomic.AddUint64(&pr.SequenceID, 1)
	return uint64ToId(counter)
}

func (pr *ProcessRegistryValue) Add(process Process, id string) (*PID, bool) {
	return &PID{
		Address: pr.Address,
		Id:      id,
	}, pr.LocalPIDs.SetIfAbsent(id, process)
}

func (pr *ProcessRegistryValue) Remove(pid *PID) {
	ref, _ := pr.LocalPIDs.Pop(pid.Id)
	if l, ok := ref.(*ActorProcess); ok {
		atomic.StoreInt32(&l.dead, 1)
	}
}

func (pr *ProcessRegistryValue) Get(pid *PID) (Process, bool) {
	if pid == nil {
		return deadLetter, false
	}
	if pid.Address != localAddress && pid.Address != pr.Address {
		for _, handler := range pr.RemoteHandlers {
			ref, ok := handler(pid)
			if ok {
				return ref, true
			}
		}
		return deadLetter, false
	}
	ref, ok := pr.LocalPIDs.Get(pid.Id)
	if !ok {
		return deadLetter, false
	}
	return ref.(Process), true
}

func (pr *ProcessRegistryValue) GetLocal(id string) (Process, bool) {
	ref, ok := pr.LocalPIDs.Get(id)
	if !ok {
		return deadLetter, false
	}
	return ref.(Process), true
}
