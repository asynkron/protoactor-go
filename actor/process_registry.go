package actor

import (
	"sync/atomic"

	murmur32 "github.com/twmb/murmur3"

	cmap "github.com/orcaman/concurrent-map"
)

// ProcessRegistryValue tracks processes within an actor system.
type ProcessRegistryValue struct {
	SequenceID     uint64
	ActorSystem    *ActorSystem
	Address        string
	LocalPIDs      *SliceMap
	RemoteHandlers []AddressResolver
}

// SliceMap is a sharded map for storing local PIDs.
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

// GetBucket returns the shard bucket for the given key.
func (s *SliceMap) GetBucket(key string) cmap.ConcurrentMap {
	hash := murmur32.Sum32([]byte(key))
	index := int(hash) % len(s.LocalPIDs)

	return s.LocalPIDs[index]
}

const (
	localAddress = "nonhost"
)

// NewProcessRegistry creates a new process registry for the actor system.
func NewProcessRegistry(actorSystem *ActorSystem) *ProcessRegistryValue {
	return &ProcessRegistryValue{
		ActorSystem: actorSystem,
		Address:     localAddress,
		LocalPIDs:   newSliceMap(),
	}
}

// An AddressResolver is used to resolve remote actors
type AddressResolver func(*PID) (Process, bool)

// RegisterAddressResolver adds a resolver for remote addresses.
func (pr *ProcessRegistryValue) RegisterAddressResolver(handler AddressResolver) {
	pr.RemoteHandlers = append(pr.RemoteHandlers, handler)
}

const (
	digits = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ~+"
)

func uint64ToID(u uint64) string {
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

// NextID returns the next unique process identifier.
func (pr *ProcessRegistryValue) NextID() string {
	counter := atomic.AddUint64(&pr.SequenceID, 1)

	return uint64ToID(counter)
}

// Add registers a process with the given id and returns its PID.
func (pr *ProcessRegistryValue) Add(process Process, id string) (*PID, bool) {
	bucket := pr.LocalPIDs.GetBucket(id)

	return &PID{
		Address: pr.Address,
		Id:      id,
	}, bucket.SetIfAbsent(id, process)
}

// Remove deletes the process with the given PID from the registry.
func (pr *ProcessRegistryValue) Remove(pid *PID) {
	bucket := pr.LocalPIDs.GetBucket(pid.Id)

	ref, _ := bucket.Pop(pid.Id)
	if l, ok := ref.(*ActorProcess); ok {
		atomic.StoreInt32(&l.dead, 1)
	}
}

// Get retrieves a process by PID, checking remote handlers if needed.
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

// GetLocal retrieves a local process by id.
func (pr *ProcessRegistryValue) GetLocal(id string) (Process, bool) {
	bucket := pr.LocalPIDs.GetBucket(id)
	ref, ok := bucket.Get(id)

	if !ok {
		return pr.ActorSystem.DeadLetter, false
	}

	return ref.(Process), true
}
