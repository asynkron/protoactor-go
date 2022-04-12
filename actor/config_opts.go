package actor

import (
	"time"

	"go.opentelemetry.io/otel/metric"
)

type ConfigOption func(config *Config)

func Configure(options ...ConfigOption) *Config {
	config := defaultConfig()
	for _, option := range options {
		option(config)
	}

	return config
}

func WithDeadLetterThrottleInterval(duration time.Duration) ConfigOption {
	return func(config *Config) {
		config.DeadLetterThrottleInterval = duration
	}
}

func WithDeadLetterThrottleCount(count int32) ConfigOption {
	return func(config *Config) {
		config.DeadLetterThrottleCount = count
	}
}

func WithDeadLetterRequestLogging(enabled bool) ConfigOption {
	return func(config *Config) {
		config.DeadLetterRequestLogging = enabled
	}
}

func WithDeveloperSupervisionLogging(enabled bool) ConfigOption {
	return func(config *Config) {
		config.DeveloperSupervisionLogging = enabled
	}
}

func WithDiagnosticsSerializer(serializer func(Actor) string) ConfigOption {
	return func(config *Config) {
		config.DiagnosticsSerializer = serializer
	}
}

func WithMetricProviders(provider metric.MeterProvider) ConfigOption {
	return func(config *Config) {
		config.MetricsProvider = provider
	}
}

func WithDefaultPrometheusProvider(port ...int) ConfigOption {
	_port := 2222
	if len(port) > 0 {
		_port = port[0]
	}

	return WithMetricProviders(defaultPrometheusProvider(_port))
}
