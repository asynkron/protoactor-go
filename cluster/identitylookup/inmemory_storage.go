package identitylookup

import (
	"sync"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/google/uuid"
)

// InMemoryStorageLookup is a simple in-memory implementation of
// cluster.StorageLookup, primarily intended for testing and for running
// the StorageConformanceSuite. It is NOT suitable for production use.
type InMemoryStorageLookup struct {
	mu          sync.Mutex
	locks       map[string]*cluster.SpawnLock               // identityKey -> SpawnLock
	activations map[string]*cluster.StoredActivation        // identityKey -> StoredActivation
	waiters     map[string][]chan *cluster.StoredActivation // identityKey -> waiting channels
}

// NewInMemoryStorageLookup creates a new InMemoryStorageLookup.
func NewInMemoryStorageLookup() *InMemoryStorageLookup {
	return &InMemoryStorageLookup{
		locks:       make(map[string]*cluster.SpawnLock),
		activations: make(map[string]*cluster.StoredActivation),
		waiters:     make(map[string][]chan *cluster.StoredActivation),
	}
}

func (s *InMemoryStorageLookup) TryGetExistingActivation(clusterIdentity *cluster.ClusterIdentity) *cluster.StoredActivation {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := identityKey(clusterIdentity)
	return s.activations[key]
}

func (s *InMemoryStorageLookup) TryAcquireLock(clusterIdentity *cluster.ClusterIdentity) *cluster.SpawnLock {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := identityKey(clusterIdentity)

	// If there's already a lock or activation for this identity, fail.
	if _, locked := s.locks[key]; locked {
		return nil
	}
	if _, activated := s.activations[key]; activated {
		return nil
	}

	lock := &cluster.SpawnLock{
		LockID:          uuid.New().String(),
		ClusterIdentity: clusterIdentity,
	}
	s.locks[key] = lock
	return lock
}

func (s *InMemoryStorageLookup) WaitForActivation(clusterIdentity *cluster.ClusterIdentity) *cluster.StoredActivation {
	s.mu.Lock()

	key := identityKey(clusterIdentity)

	// If activation already exists, return immediately.
	if act, ok := s.activations[key]; ok {
		s.mu.Unlock()
		return act
	}

	// Otherwise register a waiter channel and block until notified.
	ch := make(chan *cluster.StoredActivation, 1)
	s.waiters[key] = append(s.waiters[key], ch)
	s.mu.Unlock()

	// Block with a timeout to prevent tests from hanging forever.
	select {
	case act := <-ch:
		return act
	case <-time.After(10 * time.Second):
		return nil
	}
}

func (s *InMemoryStorageLookup) RemoveLock(spawnLock cluster.SpawnLock) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := identityKey(spawnLock.ClusterIdentity)
	if existing, ok := s.locks[key]; ok && existing.LockID == spawnLock.LockID {
		delete(s.locks, key)
	}
}

func (s *InMemoryStorageLookup) StoreActivation(memberID string, spawnLock *cluster.SpawnLock, pid *actor.PID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := identityKey(spawnLock.ClusterIdentity)

	// Remove the lock since the activation is now stored.
	delete(s.locks, key)

	activation := &cluster.StoredActivation{
		Pid:      pid.Address + "/" + pid.Id,
		MemberID: memberID,
	}
	s.activations[key] = activation

	// Notify any waiters.
	for _, ch := range s.waiters[key] {
		select {
		case ch <- activation:
		default:
		}
	}
	delete(s.waiters, key)
}

func (s *InMemoryStorageLookup) RemoveActivation(spawnLock *cluster.SpawnLock) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := identityKey(spawnLock.ClusterIdentity)
	delete(s.activations, key)
}

func (s *InMemoryStorageLookup) RemoveMemberId(memberID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, act := range s.activations {
		if act.MemberID == memberID {
			delete(s.activations, key)
		}
	}
}

// Compile-time check that InMemoryStorageLookup implements StorageGrainEnumerator.
var _ cluster.StorageGrainEnumerator = (*InMemoryStorageLookup)(nil)

func (s *InMemoryStorageLookup) ListActivations() ([]*cluster.StoredActivationInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]*cluster.StoredActivationInfo, 0, len(s.activations))
	for key, act := range s.activations {
		kind, identity := parseIdentityKey(key)
		result = append(result, &cluster.StoredActivationInfo{
			Identity: identity,
			Kind:     kind,
			Pid:      act.Pid,
			MemberID: act.MemberID,
		})
	}
	return result, nil
}

func (s *InMemoryStorageLookup) ListActivationsByMember(memberID string) ([]*cluster.StoredActivationInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []*cluster.StoredActivationInfo
	for key, act := range s.activations {
		if act.MemberID == memberID {
			kind, identity := parseIdentityKey(key)
			result = append(result, &cluster.StoredActivationInfo{
				Identity: identity,
				Kind:     kind,
				Pid:      act.Pid,
				MemberID: act.MemberID,
			})
		}
	}
	return result, nil
}

// parseIdentityKey splits a "kind/identity" key into its components.
func parseIdentityKey(key string) (string, string) {
	for i := 0; i < len(key); i++ {
		if key[i] == '/' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}
