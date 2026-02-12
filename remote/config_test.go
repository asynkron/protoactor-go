package remote

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoteConfigValidate_ValidConfig(t *testing.T) {
	config := defaultConfig()
	config.Host = "localhost"
	config.Port = 8080
	err := config.validate()
	assert.NoError(t, err)
}

func TestRemoteConfigValidate_EmptyHost(t *testing.T) {
	config := defaultConfig()
	config.Host = ""
	config.Port = 8080
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host must not be empty")
}

func TestRemoteConfigValidate_NegativePort(t *testing.T) {
	config := defaultConfig()
	config.Host = "localhost"
	config.Port = -1
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port must be >= 0")
}

func TestRemoteConfigValidate_ZeroPort(t *testing.T) {
	config := defaultConfig()
	config.Host = "localhost"
	config.Port = 0
	err := config.validate()
	assert.NoError(t, err, "port 0 should be valid (OS assigns ephemeral port)")
}

func TestRemoteConfigValidate_ZeroEndpointWriterBatchSize(t *testing.T) {
	config := defaultConfig()
	config.Host = "localhost"
	config.Port = 8080
	config.EndpointWriterBatchSize = 0
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EndpointWriterBatchSize must be > 0")
}

func TestRemoteConfigValidate_ZeroEndpointWriterQueueSize(t *testing.T) {
	config := defaultConfig()
	config.Host = "localhost"
	config.Port = 8080
	config.EndpointWriterQueueSize = 0
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EndpointWriterQueueSize must be > 0")
}

func TestRemoteConfigValidate_ZeroEndpointManagerBatchSize(t *testing.T) {
	config := defaultConfig()
	config.Host = "localhost"
	config.Port = 8080
	config.EndpointManagerBatchSize = 0
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EndpointManagerBatchSize must be > 0")
}

func TestRemoteConfigValidate_ZeroEndpointManagerQueueSize(t *testing.T) {
	config := defaultConfig()
	config.Host = "localhost"
	config.Port = 8080
	config.EndpointManagerQueueSize = 0
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EndpointManagerQueueSize must be > 0")
}

func TestConfigureWithError_ValidConfig(t *testing.T) {
	config, err := ConfigureWithError("localhost", 8080)
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 8080, config.Port)
}

func TestConfigureWithError_EmptyHost(t *testing.T) {
	config, err := ConfigureWithError("", 8080)
	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestConfigureWithError_NegativePort(t *testing.T) {
	config, err := ConfigureWithError("localhost", -1)
	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestConfigure_PanicsOnEmptyHost(t *testing.T) {
	assert.Panics(t, func() {
		Configure("", 8080)
	})
}

func TestConfigure_PanicsOnNegativePort(t *testing.T) {
	assert.Panics(t, func() {
		Configure("localhost", -1)
	})
}

func TestConfigure_ValidConfig(t *testing.T) {
	config := Configure("localhost", 8080)
	assert.NotNil(t, config)
	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 8080, config.Port)
}

func TestConfigureWithError_WithOptions(t *testing.T) {
	config, err := ConfigureWithError("localhost", 9090,
		WithEndpointWriterBatchSize(500),
		WithEndpointWriterQueueSize(50000),
	)
	require.NoError(t, err)
	assert.Equal(t, 500, config.EndpointWriterBatchSize)
	assert.Equal(t, 50000, config.EndpointWriterQueueSize)
}

func TestRemoteConfig_DefaultRetryBaseDelay(t *testing.T) {
	config := defaultConfig()
	assert.Equal(t, 2*time.Second, config.RetryBaseDelay)
}

func TestRemoteConfig_DefaultShutdownTimeout(t *testing.T) {
	config := defaultConfig()
	assert.Equal(t, 10*time.Second, config.ShutdownTimeout)
}

func TestRemoteConfig_WithRetryBaseDelay(t *testing.T) {
	config := Configure("localhost", 0, WithRetryBaseDelay(500*time.Millisecond))
	require.Equal(t, 500*time.Millisecond, config.RetryBaseDelay)
}

func TestRemoteConfig_WithShutdownTimeout(t *testing.T) {
	config := Configure("localhost", 0, WithShutdownTimeout(30*time.Second))
	require.Equal(t, 30*time.Second, config.ShutdownTimeout)
}

func TestRemoteConfigValidate_ZeroRetryBaseDelay(t *testing.T) {
	config := defaultConfig()
	config.Host = "localhost"
	config.Port = 8080
	config.RetryBaseDelay = 0
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RetryBaseDelay must be > 0")
}

func TestRemoteConfigValidate_NegativeRetryBaseDelay(t *testing.T) {
	config := defaultConfig()
	config.Host = "localhost"
	config.Port = 8080
	config.RetryBaseDelay = -1 * time.Second
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RetryBaseDelay must be > 0")
}

func TestRemoteConfigValidate_ZeroShutdownTimeout(t *testing.T) {
	config := defaultConfig()
	config.Host = "localhost"
	config.Port = 8080
	config.ShutdownTimeout = 0
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ShutdownTimeout must be > 0")
}

func TestRemoteConfigValidate_NegativeShutdownTimeout(t *testing.T) {
	config := defaultConfig()
	config.Host = "localhost"
	config.Port = 8080
	config.ShutdownTimeout = -1 * time.Second
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ShutdownTimeout must be > 0")
}
