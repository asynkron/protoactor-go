// Package cluster provides distributed actor clustering with virtual actors (grains) and gossip-based membership.
//
// The cluster package enables multi-node actor systems where actors can be addressed by identity
// rather than physical location. It includes support for grain-based virtual actors, pub/sub messaging,
// and gossip protocol for cluster membership and state propagation.
//
// Key types:
//   - Cluster: The main cluster instance managing membership, identity lookup, and grain activation
//   - Config: Configuration for cluster behavior including timeouts, strategies, and providers
//   - Context: Cluster-aware context for making grain calls and accessing cluster features
//   - GrainCallConfig: Options for configuring grain method calls (timeout, retries)
//   - IdentityLookup: Interface for mapping virtual actor identities to physical PIDs
//   - ClusterProvider: Interface for cluster membership provider implementations
//   - PubSub: Cluster-wide publish-subscribe messaging
//   - Gossiper: Manages gossip protocol for distributing state across the cluster
//   - MemberList: Tracks cluster members and their status
//
// Basic usage:
//
//	// Create and start a cluster
//	system := actor.NewActorSystem()
//	provider := consulprovider.New()
//	lookup := disthash.New()
//	config := cluster.NewConfig("my-cluster", provider, lookup)
//	c := cluster.NewCluster(system, config)
//	c.StartMember()
//
//	// Call a grain
//	pid := c.Get("user-123", "UserActor")
//	response, err := c.Request(pid, &GetUserRequest{}, 5*time.Second)
package cluster
