package actor

import (
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/metric"
)

// ConfigOption is a function that configures the actor system
type ConfigOption func(config *Config)

// Configure sets the configuration options
func Configure(options ...ConfigOption) *Config {
	config := defaultConfig()
	for _, option := range options {
		option(config)
	}

	return config
}

// WithDeadLetterThrottleInterval sets the dead letter throttle interval
func WithDeadLetterThrottleInterval(duration time.Duration) ConfigOption {
	return func(config *Config) {
		config.DeadLetterThrottleInterval = duration
	}
}

// WithDeadLetterThrottleCount sets the dead letter throttle count
func WithDeadLetterThrottleCount(count int32) ConfigOption {
	return func(config *Config) {
		config.DeadLetterThrottleCount = count
	}
}

// WithDeadLetterRequestLogging sets the dead letter request logging on or off
func WithDeadLetterRequestLogging(enabled bool) ConfigOption {
	return func(config *Config) {
		config.DeadLetterRequestLogging = enabled
	}
}

// WithDeveloperSupervisionLogging sets the developer supervision logging on or off
func WithDeveloperSupervisionLogging(enabled bool) ConfigOption {
	return func(config *Config) {
		config.DeveloperSupervisionLogging = enabled
	}
}

// WithDiagnosticsSerializer sets the diagnostics serializer
func WithDiagnosticsSerializer(serializer func(Actor) string) ConfigOption {
	return func(config *Config) {
		config.DiagnosticsSerializer = serializer
	}
}

// WithMetricProviders sets the metric providers
func WithMetricProviders(provider metric.MeterProvider) ConfigOption {

	return func(config *Config) {
		config.MetricsProvider = provider
	}
}

// WithDefaultPrometheusProvider sets the default prometheus provider
func WithDefaultPrometheusProvider(port ...int) ConfigOption {
	_port := 2222
	if len(port) > 0 {
		_port = port[0]
	}

	return WithMetricProviders(defaultPrometheusProvider(_port))
}

// WithLoggerFactory sets the logger factory to use for the actor system
func WithLoggerFactory(factory func(system *ActorSystem) *slog.Logger) ConfigOption {
	return func(config *Config) {
		config.LoggerFactory = factory
	}
}
