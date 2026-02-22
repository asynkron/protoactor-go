package natsstream

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := newDefaultConfig()
	assert.Equal(t, defaultHeartbeatInterval, cfg.HeartbeatInterval)
	assert.Equal(t, defaultHeartbeatTTL, cfg.HeartbeatTTL)
	assert.Equal(t, defaultMemberTimeout, cfg.MemberTimeout)
	assert.Equal(t, defaultCheckInterval, cfg.CheckInterval)
	assert.Equal(t, defaultLeaderTTL, cfg.LeaderTTL)
	assert.Equal(t, defaultLockTTL, cfg.LockTTL)
	assert.Equal(t, defaultMaxConcurrency, cfg.MaxConcurrency)
	assert.Equal(t, defaultReplicas, cfg.Replicas)
	assert.Equal(t, defaultSubjectPrefix, cfg.SubjectPrefix)
	assert.Equal(t, defaultRetryInterval, cfg.RetryInterval)
	assert.Equal(t, defaultMaxAge, cfg.MaxAge)
}

func TestWithOptions(t *testing.T) {
	cfg := newDefaultConfig()

	WithHeartbeatInterval(1 * time.Second)(cfg)
	assert.Equal(t, 1*time.Second, cfg.HeartbeatInterval)

	WithHeartbeatTTL(3 * time.Second)(cfg)
	assert.Equal(t, 3*time.Second, cfg.HeartbeatTTL)

	WithMemberTimeout(6 * time.Second)(cfg)
	assert.Equal(t, 6*time.Second, cfg.MemberTimeout)

	WithCheckInterval(2 * time.Second)(cfg)
	assert.Equal(t, 2*time.Second, cfg.CheckInterval)

	WithLeaderTTL(15 * time.Second)(cfg)
	assert.Equal(t, 15*time.Second, cfg.LeaderTTL)

	WithLockTTL(10 * time.Second)(cfg)
	assert.Equal(t, 10*time.Second, cfg.LockTTL)

	WithMaxConcurrency(100)(cfg)
	assert.Equal(t, 100, cfg.MaxConcurrency)

	WithReplicas(3)(cfg)
	assert.Equal(t, 3, cfg.Replicas)

	WithStreamName("MY_STREAM")(cfg)
	assert.Equal(t, "MY_STREAM", cfg.StreamName)

	WithIdentityStreamName("MY_IDENTITIES")(cfg)
	assert.Equal(t, "MY_IDENTITIES", cfg.IdentityStreamName)

	WithSubjectPrefix("myapp.cluster")(cfg)
	assert.Equal(t, "myapp.cluster", cfg.SubjectPrefix)

	WithRetryInterval(5 * time.Second)(cfg)
	assert.Equal(t, 5*time.Second, cfg.RetryInterval)

	WithMaxAge(2 * time.Hour)(cfg)
	assert.Equal(t, 2*time.Hour, cfg.MaxAge)
}

func TestStreamName_Default(t *testing.T) {
	cfg := newDefaultConfig()
	assert.Equal(t, "PROTOACTOR_mycluster", cfg.streamName("mycluster"))
}

func TestStreamName_Custom(t *testing.T) {
	cfg := newDefaultConfig()
	cfg.StreamName = "CUSTOM"
	assert.Equal(t, "CUSTOM", cfg.streamName("mycluster"))
}

func TestIdentityStreamName_Default(t *testing.T) {
	cfg := newDefaultConfig()
	assert.Equal(t, "PROTOACTOR_mycluster_IDENTITIES", cfg.identityStreamName("mycluster"))
}

func TestIdentityStreamName_Custom(t *testing.T) {
	cfg := newDefaultConfig()
	cfg.IdentityStreamName = "CUSTOM_ID"
	assert.Equal(t, "CUSTOM_ID", cfg.identityStreamName("mycluster"))
}

func TestSubjectPrefix_Default(t *testing.T) {
	cfg := newDefaultConfig()
	assert.Equal(t, "cluster.mycluster", cfg.subjectPrefix("mycluster"))
}

func TestSubjectPrefix_Custom(t *testing.T) {
	cfg := newDefaultConfig()
	cfg.SubjectPrefix = "myapp"
	assert.Equal(t, "myapp", cfg.subjectPrefix("mycluster"))
}

func TestRoleType_String(t *testing.T) {
	assert.Equal(t, "Follower", Follower.String())
	assert.Equal(t, "Leader", Leader.String())
}
