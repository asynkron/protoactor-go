package actor

import (
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/metric"
)

// ConfigOption is a function that configures the actor system
type ConfigOption func(config *Config)

// ConfigureWithError sets the configuration options and validates the result.
// It returns an error if the configuration is invalid.
func ConfigureWithError(options ...ConfigOption) (*Config, error) {
	config := defaultConfig()
	for _, option := range options {
		option(config)
	}

	if err := config.validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// Configure sets the configuration options. It panics if the resulting
// configuration is invalid. Use ConfigureWithError for a non-panicking variant.
func Configure(options ...ConfigOption) *Config {
	config, err := ConfigureWithError(options...)
	if err != nil {
		panic(err)
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
		config.MetricsEnabled = true
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

// WithStopTimeout sets the timeout used for StopFuture and PoisonFuture calls.
func WithStopTimeout(d time.Duration) ConfigOption {
	return func(config *Config) {
		config.StopTimeout = d
	}
}

// WithRequestTimeout sets the default timeout used for request operations.
func WithRequestTimeout(d time.Duration) ConfigOption {
	return func(config *Config) {
		config.RequestTimeout = d
	}
}
