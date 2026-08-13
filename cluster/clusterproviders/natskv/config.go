package natskv

import (
	"time"

	"github.com/awevoke/protoactor-go/cluster"
)

const (
	defaultMemberTTL             = 5 * time.Second
	defaultRefreshInterval       = 2 * time.Second
	defaultReconcileInterval     = 15 * time.Second
	defaultLeaderTTL             = 10 * time.Second
	defaultLockTTL               = 5 * time.Second
	defaultMaxConcurrency        = 200
	defaultReplicas              = 1
	defaultKeyPrefix             = "cluster"
	defaultRetryInterval         = 1 * time.Second
	defaultLockOwnerAbsentGrace  = 30 * time.Second
	defaultHardReapAge           = 60 * time.Second
	defaultActivationAbsentGrace = 60 * time.Second
	defaultWaiterWindow          = 15 * time.Second
	defaultJanitorInterval       = 30 * time.Second
)

// config holds internal configuration for the NATS KV cluster provider.
type config struct {
	BucketName        string
	IdentityBucket    string
	KeyPrefix         string
	Replicas          int
	MemberTTL         time.Duration
	RefreshInterval   time.Duration
	ReconcileInterval time.Duration
	LeaderTTL         time.Duration
	LockTTL           time.Duration
	MaxConcurrency    int
	RetryInterval     time.Duration
	RoleChanged       cluster.RoleChangedListener

	// Reaping and janitor configuration.
	//
	// LockOwnerAbsentGrace is the duration after which a lock record whose
	// owning member is absent from the cluster may be forcibly reaped.
	LockOwnerAbsentGrace time.Duration

	// HardReapAge is the maximum age of any identity record; entries older
	// than this are eligible for forced removal regardless of member state.
	HardReapAge time.Duration

	// ActivationAbsentGrace is the time after which an activation whose
	// owning member is no longer in the cluster is eligible for cleanup.
	ActivationAbsentGrace time.Duration

	// WaiterWindow is the maximum time a node waits for an in-progress
	// activation (held lock) to resolve before treating the lock as stale.
	WaiterWindow time.Duration

	// JanitorInterval is the cadence at which the background janitor
	// scans for and removes stale identity records.
	JanitorInterval time.Duration
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

// WithReconcileInterval sets the interval at which the provider reconciles its
// in-memory member set against the live member keys in the KV bucket, pruning
// stale members whose keys have expired without a delivered delete event.
// A value <= 0 disables periodic reconciliation.
func WithReconcileInterval(interval time.Duration) Option {
	return func(c *config) { c.ReconcileInterval = interval }
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
func WithRoleChangedListener(l cluster.RoleChangedListener) Option {
	return func(c *config) { c.RoleChanged = l }
}

// WithLockOwnerAbsentGrace sets the grace period before a lock whose owning
// member is absent from the cluster may be forcibly reaped.
func WithLockOwnerAbsentGrace(d time.Duration) Option {
	return func(c *config) { c.LockOwnerAbsentGrace = d }
}

// WithHardReapAge sets the maximum age of any identity record before it is
// eligible for forced removal regardless of member state.
func WithHardReapAge(d time.Duration) Option {
	return func(c *config) { c.HardReapAge = d }
}

// WithActivationAbsentGrace sets the time after which an activation whose
// owning member is no longer in the cluster is eligible for cleanup.
func WithActivationAbsentGrace(d time.Duration) Option {
	return func(c *config) { c.ActivationAbsentGrace = d }
}

// WithWaiterWindow sets the maximum time a node waits for an in-progress
// activation to resolve before treating the lock as stale.
func WithWaiterWindow(d time.Duration) Option {
	return func(c *config) { c.WaiterWindow = d }
}

// WithJanitorInterval sets the cadence at which the background janitor
// scans for and removes stale identity records. A value <= 0 disables
// the janitor.
func WithJanitorInterval(d time.Duration) Option {
	return func(c *config) { c.JanitorInterval = d }
}

func newDefaultConfig() *config {
	return &config{
		KeyPrefix:             defaultKeyPrefix,
		Replicas:              defaultReplicas,
		MemberTTL:             defaultMemberTTL,
		RefreshInterval:       defaultRefreshInterval,
		ReconcileInterval:     defaultReconcileInterval,
		LeaderTTL:             defaultLeaderTTL,
		LockTTL:               defaultLockTTL,
		MaxConcurrency:        defaultMaxConcurrency,
		RetryInterval:         defaultRetryInterval,
		LockOwnerAbsentGrace:  defaultLockOwnerAbsentGrace,
		HardReapAge:           defaultHardReapAge,
		ActivationAbsentGrace: defaultActivationAbsentGrace,
		WaiterWindow:          defaultWaiterWindow,
		JanitorInterval:       defaultJanitorInterval,
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
