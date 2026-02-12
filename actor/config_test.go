package actor

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate_DefaultConfigIsValid(t *testing.T) {
	config := defaultConfig()
	err := config.validate()
	assert.NoError(t, err)
}

func TestConfigValidate_NegativeDeadLetterThrottleCount(t *testing.T) {
	config := defaultConfig()
	config.DeadLetterThrottleCount = -1
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DeadLetterThrottleCount must be >= 0")
}

func TestConfigValidate_ZeroDeadLetterThrottleInterval(t *testing.T) {
	config := defaultConfig()
	config.DeadLetterThrottleInterval = 0
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DeadLetterThrottleInterval must be > 0")
}

func TestConfigValidate_NegativeDeadLetterThrottleInterval(t *testing.T) {
	config := defaultConfig()
	config.DeadLetterThrottleInterval = -1 * time.Second
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DeadLetterThrottleInterval must be > 0")
}

func TestConfigValidate_NilLoggerFactory(t *testing.T) {
	config := defaultConfig()
	config.LoggerFactory = nil
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LoggerFactory must not be nil")
}

func TestConfigureWithError_ValidConfig(t *testing.T) {
	config, err := ConfigureWithError()
	assert.NoError(t, err)
	assert.NotNil(t, config)
}

func TestConfigureWithError_InvalidConfig(t *testing.T) {
	config, err := ConfigureWithError(func(c *Config) {
		c.DeadLetterThrottleCount = -5
	})
	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestNewActorSystemWithError_ValidConfig(t *testing.T) {
	sys, err := NewActorSystemWithError()
	require.NoError(t, err)
	require.NotNil(t, sys)
	defer sys.Shutdown()
	assert.NotEmpty(t, sys.ID)
}

func TestNewActorSystemWithError_InvalidConfig(t *testing.T) {
	sys, err := NewActorSystemWithError(func(c *Config) {
		c.DeadLetterThrottleCount = -1
	})
	assert.Error(t, err)
	assert.Nil(t, sys)
}

func TestNewActorSystem_PanicsOnInvalidConfig(t *testing.T) {
	assert.Panics(t, func() {
		NewActorSystem(func(c *Config) {
			c.DeadLetterThrottleCount = -1
		})
	})
}

func TestConfigure_PanicsOnInvalidConfig(t *testing.T) {
	assert.Panics(t, func() {
		Configure(func(c *Config) {
			c.LoggerFactory = nil
		})
	})
}

func TestConfigValidate_ZeroDeadLetterThrottleCountIsValid(t *testing.T) {
	config := defaultConfig()
	config.DeadLetterThrottleCount = 0
	err := config.validate()
	assert.NoError(t, err)
}

func TestConfigValidate_MultipleErrors_ReportsFirst(t *testing.T) {
	config := defaultConfig()
	config.DeadLetterThrottleCount = -1
	config.DeadLetterThrottleInterval = 0
	config.LoggerFactory = nil
	err := config.validate()
	require.Error(t, err)
	// First validation check should be DeadLetterThrottleCount
	assert.True(t, strings.Contains(err.Error(), "DeadLetterThrottleCount"))
}

func TestActorConfig_DefaultStopTimeout(t *testing.T) {
	config := defaultConfig()
	assert.Equal(t, 10*time.Second, config.StopTimeout)
}

func TestActorConfig_DefaultRequestTimeout(t *testing.T) {
	config := defaultConfig()
	assert.Equal(t, 5*time.Second, config.RequestTimeout)
}

func TestActorConfig_WithStopTimeout(t *testing.T) {
	sys := NewActorSystem(WithStopTimeout(5 * time.Second))
	defer sys.Shutdown()
	require.Equal(t, 5*time.Second, sys.Config.StopTimeout)
}

func TestActorConfig_WithRequestTimeout(t *testing.T) {
	sys := NewActorSystem(WithRequestTimeout(3 * time.Second))
	defer sys.Shutdown()
	require.Equal(t, 3*time.Second, sys.Config.RequestTimeout)
}

func TestConfigValidate_ZeroStopTimeout(t *testing.T) {
	config := defaultConfig()
	config.StopTimeout = 0
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "StopTimeout must be > 0")
}

func TestConfigValidate_NegativeStopTimeout(t *testing.T) {
	config := defaultConfig()
	config.StopTimeout = -1 * time.Second
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "StopTimeout must be > 0")
}

func TestConfigValidate_ZeroRequestTimeout(t *testing.T) {
	config := defaultConfig()
	config.RequestTimeout = 0
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RequestTimeout must be > 0")
}

func TestConfigValidate_NegativeRequestTimeout(t *testing.T) {
	config := defaultConfig()
	config.RequestTimeout = -1 * time.Second
	err := config.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RequestTimeout must be > 0")
}
