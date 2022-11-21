package actor

import (
	"sync/atomic"

	murmur32 "github.com/twmb/murmur3"

	cmap "github.com/orcaman/concurrent-map"
)

type ProcessRegistryValue struct {
	SequenceID     uint64
	ActorSystem    *ActorSystem
	Address        string
	LocalPIDs      *SliceMap
	RemoteHandlers []AddressResolver
}

type SliceMap struct {
	LocalPIDs []cmap.ConcurrentMap
}

func newSliceMap() *SliceMap {
	sm := &SliceMap{}
	sm.LocalPIDs = make([]cmap.ConcurrentMap, 1024)

	for i := 0; i < len(sm.LocalPIDs); i++ {
		sm.LocalPIDs[i] = cmap.New()
	}

	return sm
}

func (s *SliceMap) GetBucket(key string) cmap.ConcurrentMap {
	hash := murmur32.Sum32([]byte(key))
	index := int(hash) % len(s.LocalPIDs)

	return s.LocalPIDs[index]
}

const (
	localAddress = "nonhost"
)

func NewProcessRegistry(actorSystem *ActorSystem) *ProcessRegistryValue {
	return &ProcessRegistryValue{
		ActorSystem: actorSystem,
		Address:     localAddress,
		LocalPIDs:   newSliceMap(),
	}
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
	bucket := pr.LocalPIDs.GetBucket(id)

	return &PID{
		Address: pr.Address,
		Id:      id,
	}, bucket.SetIfAbsent(id, process)
}

func (pr *ProcessRegistryValue) Remove(pid *PID) {
	bucket := pr.LocalPIDs.GetBucket(pid.Id)

	ref, _ := bucket.Pop(pid.Id)
	if l, ok := ref.(*ActorProcess); ok {
		atomic.StoreInt32(&l.dead, 1)
	}
}

func (pr *ProcessRegistryValue) Get(pid *PID) (Process, bool) {
	if pid == nil {
		return pr.ActorSystem.DeadLetter, false
	}

	if pid.Address != localAddress && pid.Address != pr.Address {
		for _, handler := range pr.RemoteHandlers {
			ref, ok := handler(pid)
			if ok {
				return ref, true
			}
		}

		return pr.ActorSystem.DeadLetter, false
	}

	bucket := pr.LocalPIDs.GetBucket(pid.Id)
	ref, ok := bucket.Get(pid.Id)

	if !ok {
		return pr.ActorSystem.DeadLetter, false
	}

	return ref.(Process), true
}

func (pr *ProcessRegistryValue) GetLocal(id string) (Process, bool) {
	bucket := pr.LocalPIDs.GetBucket(id)
	ref, ok := bucket.Get(id)

	if !ok {
		return pr.ActorSystem.DeadLetter, false
	}

	return ref.(Process), true
}
