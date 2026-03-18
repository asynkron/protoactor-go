package natskv

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/cluster"
	"github.com/stretchr/testify/assert"
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
