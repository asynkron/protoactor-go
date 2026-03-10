package identitylookup_test

import (
	"testing"

	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/identitylookup"
)

// TestInMemoryConformance runs the full StorageLookup conformance suite
// against the in-memory implementation, proving the suite itself works.
func TestInMemoryConformance(t *testing.T) {
	suite := &identitylookup.StorageConformanceSuite{
		NewStorage: func() cluster.StorageLookup {
			return identitylookup.NewInMemoryStorageLookup()
		},
	}
	suite.RunAll(t)
}

// TestInMemoryEnumeratorConformance runs the StorageGrainEnumerator
// conformance suite against the in-memory implementation.
func TestInMemoryEnumeratorConformance(t *testing.T) {
	suite := &identitylookup.EnumeratorConformanceSuite{
		NewStorage: func() identitylookup.EnumerableStorage {
			return identitylookup.NewInMemoryStorageLookup()
		},
	}
	suite.RunAll(t)
}
