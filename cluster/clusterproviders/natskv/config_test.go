package natskv

import (
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := newDefaultConfig()

	assert.Equal(t, defaultKeyPrefix, cfg.KeyPrefix)
	assert.Equal(t, defaultReplicas, cfg.Replicas)
	assert.Equal(t, defaultMemberTTL, cfg.MemberTTL)
	assert.Equal(t, defaultRefreshInterval, cfg.RefreshInterval)
	assert.Equal(t, defaultLeaderTTL, cfg.LeaderTTL)
	assert.Equal(t, defaultLockTTL, cfg.LockTTL)
	assert.Equal(t, defaultMaxConcurrency, cfg.MaxConcurrency)
	assert.Equal(t, defaultRetryInterval, cfg.RetryInterval)
	assert.Empty(t, cfg.BucketName)
	assert.Empty(t, cfg.IdentityBucket)
	assert.Nil(t, cfg.RoleChanged)
}

func TestFailStopConfigDefaults(t *testing.T) {
	cfg := newDefaultConfig()
	assert.Equal(t, 15, cfg.WriteFailureThreshold)
	assert.Equal(t, 90*time.Second, cfg.WriteFailureWindow)
	assert.Equal(t, 12*time.Second, cfg.RemoteActivationTimeout)
	assert.Nil(t, cfg.FailStop, "default FailStop is nil; the recorder falls back to defaultFailStop")
}

func TestWriteFailureOptions(t *testing.T) {
	cfg := newDefaultConfig()
	WithWriteFailureThreshold(42)(cfg)
	WithWriteFailureWindow(5 * time.Second)(cfg)
	WithRemoteActivationTimeout(20 * time.Second)(cfg)
	assert.Equal(t, 42, cfg.WriteFailureThreshold)
	assert.Equal(t, 5*time.Second, cfg.WriteFailureWindow)
	assert.Equal(t, 20*time.Second, cfg.RemoteActivationTimeout)

	// Non-positive values leave defaults untouched.
	WithWriteFailureThreshold(0)(cfg)
	WithWriteFailureWindow(-1)(cfg)
	WithRemoteActivationTimeout(0)(cfg)
	assert.Equal(t, 42, cfg.WriteFailureThreshold)
	assert.Equal(t, 5*time.Second, cfg.WriteFailureWindow)
	assert.Equal(t, 20*time.Second, cfg.RemoteActivationTimeout)
}

func TestWithFailStopDisabled(t *testing.T) {
	cfg := newDefaultConfig()
	WithFailStopDisabled()(cfg)
	require.NotNil(t, cfg.FailStop)
	cfg.FailStop("noop") // must not panic or exit
}

func TestWithFailStop(t *testing.T) {
	cfg := newDefaultConfig()
	called := ""
	WithFailStop(func(r string) { called = r })(cfg)
	cfg.FailStop("boom")
	assert.Equal(t, "boom", called)
}

func TestWithBucketName(t *testing.T) {
	cfg := newDefaultConfig()
	WithBucketName("my-bucket")(cfg)
	assert.Equal(t, "my-bucket", cfg.BucketName)
}

func TestWithIdentityBucket(t *testing.T) {
	cfg := newDefaultConfig()
	WithIdentityBucket("id-bucket")(cfg)
	assert.Equal(t, "id-bucket", cfg.IdentityBucket)
}

func TestWithKeyPrefix(t *testing.T) {
	cfg := newDefaultConfig()
	WithKeyPrefix("custom-prefix")(cfg)
	assert.Equal(t, "custom-prefix", cfg.KeyPrefix)
}

func TestWithMemberTTL(t *testing.T) {
	cfg := newDefaultConfig()
	WithMemberTTL(10 * time.Second)(cfg)
	assert.Equal(t, 10*time.Second, cfg.MemberTTL)
}

func TestWithRefreshInterval(t *testing.T) {
	cfg := newDefaultConfig()
	WithRefreshInterval(3 * time.Second)(cfg)
	assert.Equal(t, 3*time.Second, cfg.RefreshInterval)
}

func TestWithLeaderTTL(t *testing.T) {
	cfg := newDefaultConfig()
	WithLeaderTTL(20 * time.Second)(cfg)
	assert.Equal(t, 20*time.Second, cfg.LeaderTTL)
}

func TestWithLockTTL(t *testing.T) {
	cfg := newDefaultConfig()
	WithLockTTL(8 * time.Second)(cfg)
	assert.Equal(t, 8*time.Second, cfg.LockTTL)
}

func TestWithMaxConcurrency(t *testing.T) {
	cfg := newDefaultConfig()
	WithMaxConcurrency(500)(cfg)
	assert.Equal(t, 500, cfg.MaxConcurrency)
}

func TestWithReplicas(t *testing.T) {
	cfg := newDefaultConfig()
	WithReplicas(3)(cfg)
	assert.Equal(t, 3, cfg.Replicas)
}

func TestWithRetryInterval(t *testing.T) {
	cfg := newDefaultConfig()
	WithRetryInterval(5 * time.Second)(cfg)
	assert.Equal(t, 5*time.Second, cfg.RetryInterval)
}

func TestWithRoleChangedListener(t *testing.T) {
	cfg := newDefaultConfig()
	listener := &mockRoleChangedListener{}
	WithRoleChangedListener(listener)(cfg)
	assert.Equal(t, listener, cfg.RoleChanged)
}

func TestMemberBucketName_Default(t *testing.T) {
	cfg := newDefaultConfig()
	assert.Equal(t, "protoactor_mycluster_members", cfg.memberBucketName("mycluster"))
}

func TestMemberBucketName_Custom(t *testing.T) {
	cfg := newDefaultConfig()
	WithBucketName("custom-members")(cfg)
	assert.Equal(t, "custom-members", cfg.memberBucketName("mycluster"))
}

func TestIdentityBucketName_Default(t *testing.T) {
	cfg := newDefaultConfig()
	assert.Equal(t, "protoactor_mycluster_identities", cfg.identityBucketName("mycluster"))
}

func TestIdentityBucketName_Custom(t *testing.T) {
	cfg := newDefaultConfig()
	WithIdentityBucket("custom-identities")(cfg)
	assert.Equal(t, "custom-identities", cfg.identityBucketName("mycluster"))
}

// mockRoleChangedListener is a test helper that implements cluster.RoleChangedListener.
type mockRoleChangedListener struct {
	lastRole cluster.RoleType
}

func (m *mockRoleChangedListener) OnRoleChanged(role cluster.RoleType) {
	m.lastRole = role
}

// TestTombstoneTTLConfig pins the delete-marker retention knob: its default,
// the option that overrides it, the opt-out, and the server-imposed floor.
func TestTombstoneTTLConfig(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		cfg := newDefaultConfig()
		assert.Equal(t, defaultTombstoneTTL, cfg.TombstoneTTL)
		assert.Equal(t, time.Hour, cfg.TombstoneTTL,
			"one hour: far longer than any reader that could race a delete, "+
				"short enough that the marker set tracks grain churn not uptime")
	})

	t.Run("override", func(t *testing.T) {
		cfg := newDefaultConfig()
		WithTombstoneTTL(90 * time.Minute)(cfg)
		assert.Equal(t, 90*time.Minute, cfg.TombstoneTTL)
	})

	t.Run("non-positive disables marker TTLs", func(t *testing.T) {
		cfg := newDefaultConfig()
		WithTombstoneTTL(0)(cfg)
		assert.Zero(t, cfg.TombstoneTTL)

		cfg = newDefaultConfig()
		WithTombstoneTTL(-time.Second)(cfg)
		assert.Zero(t, cfg.TombstoneTTL, "a negative TTL is an opt-out, not an invalid stream config")
	})

	t.Run("clamped to the server floor", func(t *testing.T) {
		// nats-server rejects SubjectDeleteMarkerTTL below one second
		// (server/stream.go:1774) with JSStreamInvalidConfig, which is NOT
		// ErrLimitMarkerTTLNotSupported and would therefore fail Setup rather
		// than fall back. Clamping keeps a mis-set knob from becoming a
		// startup outage.
		cfg := newDefaultConfig()
		WithTombstoneTTL(50 * time.Millisecond)(cfg)
		assert.Equal(t, minTombstoneTTL, cfg.TombstoneTTL)
		assert.Equal(t, time.Second, minTombstoneTTL)
	})
}
