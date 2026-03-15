package cluster

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterConfigValidate_NilClusterProvider(t *testing.T) {
	config := &Config{
		Name:               "test-cluster",
		ClusterProvider:    nil,
		IdentityLookup:     &fakeIdentityLookup{},
		RemoteConfig:       remote.Configure("localhost", 0),
		RequestTimeoutTime: defaultActorRequestTimeout,
	}
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ClusterProvider must not be nil")
}

func TestClusterConfigValidate_NilIdentityLookup(t *testing.T) {
	config := &Config{
		Name:               "test-cluster",
		ClusterProvider:    newInmemoryProvider(),
		IdentityLookup:     nil,
		RemoteConfig:       remote.Configure("localhost", 0),
		RequestTimeoutTime: defaultActorRequestTimeout,
	}
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "IdentityLookup must not be nil")
}

func TestClusterConfigValidate_NilRemoteConfig(t *testing.T) {
	config := &Config{
		Name:               "test-cluster",
		ClusterProvider:    newInmemoryProvider(),
		IdentityLookup:     &fakeIdentityLookup{},
		RemoteConfig:       nil,
		RequestTimeoutTime: defaultActorRequestTimeout,
	}
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RemoteConfig must not be nil")
}

func TestClusterConfigValidate_EmptyName(t *testing.T) {
	config := &Config{
		Name:               "",
		ClusterProvider:    newInmemoryProvider(),
		IdentityLookup:     &fakeIdentityLookup{},
		RemoteConfig:       remote.Configure("localhost", 0),
		RequestTimeoutTime: defaultActorRequestTimeout,
	}
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cluster name must not be empty")
}

func TestClusterConfigValidate_ValidConfig(t *testing.T) {
	config := &Config{
		Name:               "test-cluster",
		ClusterProvider:    newInmemoryProvider(),
		IdentityLookup:     &fakeIdentityLookup{},
		RemoteConfig:       remote.Configure("localhost", 0),
		RequestTimeoutTime: defaultActorRequestTimeout,
	}
	err := config.validate()
	assert.NoError(t, err)
}

func TestClusterConfigureWithError_ValidConfig(t *testing.T) {
	provider := newInmemoryProvider()
	lookup := &fakeIdentityLookup{}
	rc := remote.Configure("localhost", 0)
	config, err := ConfigureWithError("test-cluster", provider, lookup, rc)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "test-cluster", config.Name)
}

func TestClusterConfigureWithError_NilProvider(t *testing.T) {
	lookup := &fakeIdentityLookup{}
	rc := remote.Configure("localhost", 0)
	config, err := ConfigureWithError("test-cluster", nil, lookup, rc)
	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestClusterConfigureWithError_NilIdentityLookup(t *testing.T) {
	provider := newInmemoryProvider()
	rc := remote.Configure("localhost", 0)
	config, err := ConfigureWithError("test-cluster", provider, nil, rc)
	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestClusterConfigure_PanicsOnNilProvider(t *testing.T) {
	lookup := &fakeIdentityLookup{}
	rc := remote.Configure("localhost", 0)
	assert.Panics(t, func() {
		Configure("test-cluster", nil, lookup, rc)
	})
}

func TestClusterConfigure_PanicsOnNilIdentityLookup(t *testing.T) {
	provider := newInmemoryProvider()
	rc := remote.Configure("localhost", 0)
	assert.Panics(t, func() {
		Configure("test-cluster", provider, nil, rc)
	})
}

func TestClusterConfig_WithGossipInterval(t *testing.T) {
	provider := newInmemoryProvider()
	lookup := &fakeIdentityLookup{}
	rc := remote.Configure("localhost", 0)
	config := Configure("test-cluster", provider, lookup, rc,
		WithGossipInterval(1*time.Second),
	)
	assert.Equal(t, 1*time.Second, config.GossipInterval)
}

func TestClusterConfig_WithGossipRequestTimeout(t *testing.T) {
	provider := newInmemoryProvider()
	lookup := &fakeIdentityLookup{}
	rc := remote.Configure("localhost", 0)
	config := Configure("test-cluster", provider, lookup, rc,
		WithGossipRequestTimeout(2*time.Second),
	)
	assert.Equal(t, 2*time.Second, config.GossipRequestTimeout)
}

func TestClusterConfig_WithGossipFanOut(t *testing.T) {
	provider := newInmemoryProvider()
	lookup := &fakeIdentityLookup{}
	rc := remote.Configure("localhost", 0)
	config := Configure("test-cluster", provider, lookup, rc,
		WithGossipFanOut(5),
	)
	assert.Equal(t, 5, config.GossipFanOut)
}

func TestClusterConfig_WithGossipMaxSend(t *testing.T) {
	provider := newInmemoryProvider()
	lookup := &fakeIdentityLookup{}
	rc := remote.Configure("localhost", 0)
	config := Configure("test-cluster", provider, lookup, rc,
		WithGossipMaxSend(100),
	)
	assert.Equal(t, 100, config.GossipMaxSend)
}

func TestClusterConfig_WithMemberStrategyBuilder(t *testing.T) {
	provider := newInmemoryProvider()
	lookup := &fakeIdentityLookup{}
	rc := remote.Configure("localhost", 0)
	customBuilder := func(cluster *Cluster, kind string) MemberStrategy {
		return nil
	}
	config := Configure("test-cluster", provider, lookup, rc,
		WithMemberStrategyBuilder(customBuilder),
	)
	assert.NotNil(t, config.MemberStrategyBuilder)
}

func TestClusterConfig_WithPidCacheTTL(t *testing.T) {
	provider := newInmemoryProvider()
	lookup := &fakeIdentityLookup{}
	rc := remote.Configure("localhost", 0)
	config := Configure("test-cluster", provider, lookup, rc,
		WithPidCacheTTL(30*time.Second),
	)
	assert.Equal(t, 30*time.Second, config.PidCacheTTL)
}
