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

	// defaultTombstoneTTL is how long a deleted identity's marker is retained
	// before the server removes it. Without a TTL the marker is retained
	// forever and the identities bucket's message count grows with every
	// identity ever activated, not with the live set: 2,160 markers against
	// 4,856 live keys on the hub at soak, +46/min. One hour is far longer than
	// any reader that could race a delete -- the only reader of the identities
	// bucket that watches for delete markers is waitForActivation, bounded by
	// WaiterWindow (15s) -- and short enough that the marker set tracks grain
	// churn rather than uptime.
	defaultTombstoneTTL = time.Hour

	// minTombstoneTTL is the server's floor for a subject-delete-marker TTL
	// (nats-server server/stream.go: "subject delete marker TTL must be at
	// least 1 second"). Below it, bucket creation fails with
	// JSStreamInvalidConfig -- which is NOT ErrLimitMarkerTTLNotSupported and
	// so would NOT take createBucketWithMarkerTTL's fallback, turning a
	// mis-set knob into a startup outage. It governs EVERY LimitMarkerTTL this
	// package sets -- the identity buckets' TombstoneTTL and the provider's
	// member/leader buckets, whose marker TTL is their key TTL -- via
	// clampMarkerTTL.
	minTombstoneTTL = time.Second

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

	// TombstoneTTL is how long the server retains the delete marker a removed
	// identity leaves behind, before expiring it on its own. It is applied
	// twice: as the bucket's LimitMarkerTTL at creation, and as the per-message
	// PurgeTTL on every identity delete (casDelete, removeActivation,
	// RemovePid). A value <= 0 opts out entirely and restores the previous
	// behaviour of markers that are retained forever.
	TombstoneTTL time.Duration

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

// clampMarkerTTL raises a positive marker TTL to the server's floor and
// normalises anything non-positive to "no marker TTL". Every LimitMarkerTTL
// this package sets goes through it: the server rejects a sub-second
// SubjectDeleteMarkerTTL with JSStreamInvalidConfig, which is a bucket-creation
// failure -- i.e. a startup outage -- rather than a capability error that
// something could fall back from.
func clampMarkerTTL(d time.Duration) time.Duration {
	switch {
	case d <= 0:
		return 0
	case d < minTombstoneTTL:
		return minTombstoneTTL
	default:
		return d
	}
}

// WithTombstoneTTL sets how long the server retains a deleted identity's
// marker before expiring it. A value <= 0 disables marker expiry, restoring
// markers that are retained forever. A positive value below the server's
// one-second floor is clamped to minTombstoneTTL rather than being passed
// through, because the server rejects a shorter marker TTL outright and that
// rejection would fail Setup instead of degrading.
func WithTombstoneTTL(d time.Duration) Option {
	return func(c *config) { c.TombstoneTTL = clampMarkerTTL(d) }
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
		TombstoneTTL:            defaultTombstoneTTL,
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
