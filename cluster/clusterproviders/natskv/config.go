package natskv

import "time"

const (
	defaultMemberTTL       = 5 * time.Second
	defaultRefreshInterval = 2 * time.Second
	defaultLeaderTTL       = 10 * time.Second
	defaultLockTTL         = 5 * time.Second
	defaultMaxConcurrency  = 200
	defaultReplicas        = 1
	defaultKeyPrefix       = "cluster"
	defaultRetryInterval   = 1 * time.Second
)

// RoleType represents the leadership role of a node in the cluster.
type RoleType int

const (
	// Follower indicates the node is not the leader.
	Follower RoleType = iota
	// Leader indicates the node currently holds leadership.
	Leader
)

// String returns a human-readable representation of the role.
func (r RoleType) String() string {
	switch r {
	case Leader:
		return "Leader"
	default:
		return "Follower"
	}
}

// RoleChangedListener receives notifications when the node's leadership role changes.
type RoleChangedListener interface {
	OnRoleChanged(RoleType)
}

// config holds internal configuration for the NATS KV cluster provider.
type config struct {
	BucketName      string
	IdentityBucket  string
	KeyPrefix       string
	Replicas        int
	MemberTTL       time.Duration
	RefreshInterval time.Duration
	LeaderTTL       time.Duration
	LockTTL         time.Duration
	MaxConcurrency  int
	RetryInterval   time.Duration
	RoleChanged     RoleChangedListener
}

// Option configures the NATS KV cluster provider.
type Option func(*config)

// WithBucketName sets a custom bucket name for member registration.
func WithBucketName(name string) Option {
	return func(c *config) { c.BucketName = name }
}

// WithIdentityBucket sets a custom bucket name for identity storage.
func WithIdentityBucket(name string) Option {
	return func(c *config) { c.IdentityBucket = name }
}

// WithKeyPrefix sets the key prefix used for all keys within the buckets.
func WithKeyPrefix(prefix string) Option {
	return func(c *config) { c.KeyPrefix = prefix }
}

// WithReplicas sets the number of NATS JetStream replicas for the KV buckets.
func WithReplicas(n int) Option {
	return func(c *config) { c.Replicas = n }
}

// WithMemberTTL sets the TTL for member registration keys.
func WithMemberTTL(ttl time.Duration) Option {
	return func(c *config) { c.MemberTTL = ttl }
}

// WithRefreshInterval sets the interval at which member registrations are refreshed.
func WithRefreshInterval(interval time.Duration) Option {
	return func(c *config) { c.RefreshInterval = interval }
}

// WithLeaderTTL sets the TTL for the leader election key.
func WithLeaderTTL(ttl time.Duration) Option {
	return func(c *config) { c.LeaderTTL = ttl }
}

// WithLockTTL sets the TTL for distributed lock keys.
func WithLockTTL(ttl time.Duration) Option {
	return func(c *config) { c.LockTTL = ttl }
}

// WithMaxConcurrency sets the maximum number of concurrent NATS operations.
func WithMaxConcurrency(n int) Option {
	return func(c *config) { c.MaxConcurrency = n }
}

// WithRetryInterval sets the interval between retry attempts for failed operations.
func WithRetryInterval(interval time.Duration) Option {
	return func(c *config) { c.RetryInterval = interval }
}

// WithRoleChangedListener sets a callback for leadership role changes.
func WithRoleChangedListener(l RoleChangedListener) Option {
	return func(c *config) { c.RoleChanged = l }
}

func newDefaultConfig() *config {
	return &config{
		KeyPrefix:       defaultKeyPrefix,
		Replicas:        defaultReplicas,
		MemberTTL:       defaultMemberTTL,
		RefreshInterval: defaultRefreshInterval,
		LeaderTTL:       defaultLeaderTTL,
		LockTTL:         defaultLockTTL,
		MaxConcurrency:  defaultMaxConcurrency,
		RetryInterval:   defaultRetryInterval,
	}
}

func (c *config) memberBucketName(clusterName string) string {
	if c.BucketName != "" {
		return c.BucketName
	}
	return "protoactor_" + clusterName + "_members"
}

func (c *config) identityBucketName(clusterName string) string {
	if c.IdentityBucket != "" {
		return c.IdentityBucket
	}
	return "protoactor_" + clusterName + "_identities"
}
