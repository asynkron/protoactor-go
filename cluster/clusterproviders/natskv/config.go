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

	defaultWriteFailureThreshold = 15
	defaultWriteFailureWindow    = 90 * time.Second
	defaultRemoteActivationTO    = 12 * time.Second

	// failStopExitCode is the process exit code used by the default FailStop
	// action. 70 is BSD sysexits.h EX_SOFTWARE ("internal software error"):
	// an unambiguous, non-zero, non-generic code that lets an orchestrator
	// distinguish a self-diagnosed fail-stop from a crash (SIGSEGV) or a
	// normal exit. The process exits so the supervisor restarts it clean,
	// which re-establishes the KV write handles that failed silently.
	failStopExitCode = 70
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

	// WriteFailureThreshold is the number of consecutive non-benign identity
	// write failures required to trip the fail-stop watchdog. CAS conflicts
	// and delete-of-absent successes reset the streak and never count.
	WriteFailureThreshold int

	// WriteFailureWindow is the minimum age of the streak's first failure
	// before fail-stop may trip. It rides out transient blips: a NATS outage
	// that recovers within this window never trips the watchdog even if it
	// produced more than WriteFailureThreshold consecutive errors.
	WriteFailureWindow time.Duration

	// FailStop is invoked when the write-failure watchdog trips. A nil value
	// is the default: tripFailStop resolves it to defaultFailStop (os.Exit(70))
	// at trip time so the supervisor restarts the process and re-establishes
	// the KV write handles. Use WithFailStop to supply a custom action, or
	// WithFailStopDisabled to install a no-op (e.g. in tests).
	FailStop func(reason string)

	// RemoteActivationTimeout bounds the client-side remote activation
	// round-trip. It must exceed the member-side spawn budget (placement RPC),
	// which can legitimately take ~10s+. It is independent of LockTTL.
	RemoteActivationTimeout time.Duration
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

// WithWriteFailureThreshold sets the number of consecutive non-benign identity
// write failures required to trip the fail-stop watchdog. A value <= 0 leaves
// the default in place.
func WithWriteFailureThreshold(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.WriteFailureThreshold = n
		}
	}
}

// WithWriteFailureWindow sets the minimum age of the write-failure streak's
// first failure before fail-stop may trip. A value <= 0 leaves the default.
func WithWriteFailureWindow(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.WriteFailureWindow = d
		}
	}
}

// WithFailStop sets a custom fail-stop action, replacing the default
// os.Exit(70) behavior. The reason string describes why the watchdog tripped.
func WithFailStop(fn func(reason string)) Option {
	return func(c *config) { c.FailStop = fn }
}

// WithFailStopDisabled disables the fail-stop watchdog entirely. Write failures
// are still logged and counted, but the process is never terminated. Intended
// for tests and environments where an external supervisor is not present.
func WithFailStopDisabled() Option {
	return func(c *config) { c.FailStop = func(string) {} }
}

// WithRemoteActivationTimeout sets the client-side timeout for the remote
// activation round-trip. It must exceed the member-side spawn budget. A value
// <= 0 leaves the default in place.
func WithRemoteActivationTimeout(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.RemoteActivationTimeout = d
		}
	}
}

func newDefaultConfig() *config {
	return &config{
		KeyPrefix:               defaultKeyPrefix,
		Replicas:                defaultReplicas,
		MemberTTL:               defaultMemberTTL,
		RefreshInterval:         defaultRefreshInterval,
		ReconcileInterval:       defaultReconcileInterval,
		LeaderTTL:               defaultLeaderTTL,
		LockTTL:                 defaultLockTTL,
		MaxConcurrency:          defaultMaxConcurrency,
		RetryInterval:           defaultRetryInterval,
		LockOwnerAbsentGrace:    defaultLockOwnerAbsentGrace,
		HardReapAge:             defaultHardReapAge,
		ActivationAbsentGrace:   defaultActivationAbsentGrace,
		WaiterWindow:            defaultWaiterWindow,
		JanitorInterval:         defaultJanitorInterval,
		WriteFailureThreshold:   defaultWriteFailureThreshold,
		WriteFailureWindow:      defaultWriteFailureWindow,
		RemoteActivationTimeout: defaultRemoteActivationTO,
		// FailStop is intentionally left nil here. The nil value is a lazy
		// sentinel: tripFailStop resolves it to defaultFailStop at trip time,
		// not at Setup time, so the logger and leadership state are current
		// when the watchdog actually fires. Callers that want a custom action
		// (e.g. tests) should use WithFailStop or WithFailStopDisabled.
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
