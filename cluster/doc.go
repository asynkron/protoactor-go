// Package cluster provides distributed actor clustering with virtual actors (grains) and gossip-based membership.
//
// The cluster package enables multi-node actor systems where actors can be addressed by identity
// rather than physical location. It includes support for grain-based virtual actors, pub/sub messaging,
// and gossip protocol for cluster membership and state propagation.
//
// # Architecture Overview
//
// A cluster consists of multiple nodes (members) running Proto.Actor systems that coordinate
// through a ClusterProvider (e.g., Consul, etcd, Kubernetes) for membership discovery and a
// gossip protocol for state dissemination. Virtual actors (grains) are automatically placed
// on cluster members and can be addressed by a (kind, identity) pair without knowing their
// physical location.
//
// # Gossip Protocol
//
// The cluster uses an eventually consistent gossip protocol for propagating state across members.
// The protocol works as follows:
//
//   - A GossipActor runs on each member, managed by a Gossiper that drives periodic gossip rounds.
//   - Each gossip round (controlled by GossipInterval, default 300ms), the local member selects
//     up to GossipFanOut (default 3) random peers and sends them a GossipRequest containing
//     state deltas (changes since the last acknowledged exchange with that peer).
//   - Each member maintains a local Informer that tracks sequence numbers and committed offsets
//     per peer to send only new or changed state entries, capped at GossipMaxSend (default 50)
//     entries per round.
//   - Gossip acks are intentionally lightweight: the receiver responds immediately without
//     piggybacking full state. Reliability is achieved through fan-out and periodic re-sends
//     rather than explicit acknowledgments.
//   - Heartbeats are propagated via gossip using the "heartbeat" state key. Each round, the
//     member publishes a MemberHeartbeat containing actor statistics. If a member's heartbeat
//     is not updated within HeartbeatExpiration (default 20s), it is added to the block list
//     and treated as failed.
//   - Consensus checks can be registered on gossip keys (e.g., "topology") to detect when all
//     members agree on a value, enabling distributed coordination primitives.
//
// # Grain Placement
//
// Grains (virtual actors) are placed on cluster members using an IdentityLookup implementation.
// The built-in implementation is disthash (distributed hash):
//
//   - disthash uses rendezvous hashing (highest random weight) with FNV-1a to deterministically
//     map a (kind, identity) pair to a member address. Given the same set of members, all nodes
//     agree on placement without coordination.
//   - Each member runs a partition-activator placement actor that owns the grains hashed to
//     its address. When a grain is requested, the requester computes the owner via rendezvous
//     hash and sends an ActivationRequest to that member's placement actor.
//   - If the grain already exists, the existing PID is returned. Otherwise, the placement actor
//     spawns the grain locally and returns the new PID. Duplicate concurrent activations for
//     the same identity are detected and rejected.
//   - When cluster topology changes, the placement actor re-evaluates ownership using the new
//     member set. Grains that no longer hash to the local member are poisoned (stopped gracefully)
//     and will be re-activated on their new owner upon the next request.
//   - Alternative IdentityLookup implementations (e.g., Redis-based) can use storage-backed
//     placement for stronger consistency at the cost of additional latency.
//
// # Pub/Sub Model
//
// The cluster provides topic-based publish/subscribe messaging:
//
//   - Topics are implemented as virtual actors (TopicActor, kind "prototopic") that manage
//     subscriber lists. Subscribers can be identified by PID (direct actor reference) or
//     ClusterIdentity (virtual actor identity).
//   - Publishing sends a PubSubBatch to the TopicActor, which groups subscribers by member
//     address and dispatches a DeliverBatchRequest to a PubSubMemberDeliveryActor running
//     on each target member. This batched, member-local delivery minimizes cross-node messages.
//   - Delivery semantics are at-most-once: messages are sent without persistent queuing or
//     retry. If a subscriber is unreachable (dead letter) or times out (SubscriberTimeout,
//     default 5s), the delivery actor reports the failure back to the TopicActor, which
//     automatically unsubscribes unreachable PID-based subscribers.
//   - Subscriptions are persisted to a KeyValueStore so that topic actors can recover their
//     subscriber lists after restarts. When cluster topology changes, PID-based subscribers
//     on members that left are automatically cleaned up.
//   - Publishers interact via the Publisher interface, which sends messages through cluster
//     requests to the topic's virtual actor.
//
// # Member Lifecycle
//
// Members progress through the following states:
//
//  1. Join: A new member registers with the ClusterProvider (e.g., Consul service registration).
//     The provider detects the new member and notifies all existing members via a topology update.
//     The MemberList computes a diff (joined/left sets) and triggers memberJoin, which registers
//     the member in each relevant MemberStrategy (used for grain placement).
//
//  2. Active: The member participates in gossip, heartbeats, and grain activation. It appears
//     in the ClusterTopology with a stable TopologyHash. Members that are active are tracked
//     in the MemberSet and can receive grain activation requests.
//
//  3. Suspected/Blocked: If a member's gossip heartbeat expires (no update within
//     HeartbeatExpiration), it is added to the block list. Blocked members are filtered out of
//     the active member set on the next topology update. A member that gracefully leaves also
//     sets the "left" gossip key, causing other members to block it proactively.
//
//  4. Left: Once blocked, the member appears in the "left" set of the next ClusterTopology.
//     MemberList triggers memberLeave, removing it from all MemberStrategy instances. An
//     EndpointTerminatedEvent is published, causing the remote layer to clean up connections
//     and deliver terminated notifications to any actors watching PIDs on that member.
//
// Members that have been blocked cannot rejoin with the same member ID. The block list is
// propagated via gossip (the "topology" key carries blocked member IDs) so that all members
// converge on the same view.
//
// # Configuration Tuning
//
// Key configuration options with defaults and guidance:
//
//   - GossipInterval (default 300ms): How often gossip rounds occur. Lower values speed up
//     state convergence but increase network traffic. For large clusters (50+ nodes), consider
//     increasing to 500ms-1s.
//
//   - GossipRequestTimeout (default 500ms): Timeout for individual gossip RPC calls. Keep this
//     short to avoid blocking the gossip actor. Increase only if network latency is high.
//
//   - GossipFanOut (default 3): Number of random peers contacted per gossip round. Higher values
//     improve convergence speed and resilience to message loss at the cost of more network
//     traffic. For clusters under 10 nodes, 2-3 is sufficient. For larger clusters, 3-5.
//
//   - GossipMaxSend (default 50): Maximum number of member state entries per gossip message.
//     Increase for large clusters with many state keys; decrease if gossip messages are too
//     large for the network MTU.
//
//   - HeartbeatExpiration (default 20s): How long before a silent member is considered failed.
//     Must be significantly larger than GossipInterval * expected convergence rounds. Setting
//     this too low causes false positives; too high delays failure detection.
//
//   - RequestTimeoutTime (default 5s): Timeout for grain requests (cluster.Request calls).
//     Tune based on expected grain response latency.
//
//   - PubSubConfig.SubscriberTimeout (default 5s): Timeout when delivering pub/sub batches
//     to individual subscribers. Increase if subscribers perform heavy processing.
//
// Use the With* option functions (e.g., WithGossipInterval, WithGossipFanOut,
// WithHeartbeatExpiration) to customize these values when calling Configure or
// ConfigureWithError.
//
// # Key Types
//
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
// # Basic Usage
//
//	// Create and start a cluster
//	system := actor.NewActorSystem()
//	provider := consulprovider.New()
//	lookup := disthash.New()
//	config := cluster.Configure("my-cluster", provider, lookup, remote.Configure("localhost", 0))
//	c := cluster.NewCluster(system, config)
//	c.StartMember()
//
//	// Call a grain
//	pid := c.Get("user-123", "UserActor")
//	response, err := c.Request("user-123", "UserActor", &GetUserRequest{})
package cluster
