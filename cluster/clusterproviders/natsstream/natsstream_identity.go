package natsstream

import (
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
)

// Compile-time check that IdentityLookup implements cluster.IdentityLookup.
var _ cluster.IdentityLookup = (*IdentityLookup)(nil)

// IdentityLookup implements cluster.IdentityLookup directly using NATS JetStream
// streams for lock acquisition, activation storage, and member tracking.
type IdentityLookup struct {
	provider  *Provider
	cluster   *cluster.Cluster
	memberID  string
	isClient  bool
	config    *config
	semaphore chan struct{}
}

// newIdentityLookup creates a new IdentityLookup associated with the given provider.
func newIdentityLookup(p *Provider) *IdentityLookup {
	return &IdentityLookup{
		provider:  p,
		config:    p.config,
		semaphore: make(chan struct{}, p.config.MaxConcurrency),
	}
}

// Get resolves a cluster identity to an actor PID.
func (il *IdentityLookup) Get(ci *cluster.ClusterIdentity) *actor.PID {
	// TODO: implement in Task 6
	return nil
}

// RemovePid removes the activation for a cluster identity.
func (il *IdentityLookup) RemovePid(ci *cluster.ClusterIdentity, pid *actor.PID) {
	// TODO: implement in Task 6
}

// Setup initializes the identity lookup with the cluster context.
func (il *IdentityLookup) Setup(c *cluster.Cluster, kinds []string, isClient bool) {
	il.cluster = c
	il.memberID = c.ActorSystem.ID
	il.isClient = isClient
	// TODO: implement fully in Task 6
}

// Shutdown performs cleanup when the cluster is shutting down.
func (il *IdentityLookup) Shutdown() {
	// TODO: implement in Task 6
}
