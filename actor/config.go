package actor

import (
	"fmt"
	"github.com/lmittmann/tint"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Config holds configuration options for an ActorSystem.
type Config struct {
	DeadLetterThrottleInterval  time.Duration      // throttle deadletter logging after this interval
	DeadLetterThrottleCount     int32              // throttle deadletter logging after this count
	DeadLetterRequestLogging    bool               // do not log dead-letters with sender
	DeveloperSupervisionLogging bool               // console log and promote supervision logs to Warning level
	DiagnosticsSerializer       func(Actor) string // extract diagnostics from actor and return as string
	MetricsProvider             metric.MeterProvider
	// MetricsEnabled toggles emission of Proto.Actor metrics.
	MetricsEnabled bool
	LoggerFactory  func(system *ActorSystem) *slog.Logger
	// StopTimeout is the timeout used for StopFuture and PoisonFuture calls.
	StopTimeout time.Duration
	// RequestTimeout is the default timeout used for request operations.
	RequestTimeout time.Duration
}

func defaultConfig() *Config {
	return &Config{
		MetricsProvider:             nil,
		MetricsEnabled:              false,
		DeadLetterThrottleInterval:  1 * time.Second,
		DeadLetterThrottleCount:     3,
		DeadLetterRequestLogging:    true,
		DeveloperSupervisionLogging: false,
		DiagnosticsSerializer: func(_ Actor) string {
			return ""
		},
		LoggerFactory: func(system *ActorSystem) *slog.Logger {
			w := os.Stderr

			// create a new logger
			return slog.New(tint.NewHandler(w, &tint.Options{
				Level:      slog.LevelInfo,
				TimeFormat: time.Kitchen,
			})).With("lib", "Proto.Actor").
				With("system", system.ID)
		},
		StopTimeout:    10 * time.Second,
		RequestTimeout: 5 * time.Second,
	}
}

func defaultPrometheusProvider(port int) metric.MeterProvider {
	exporter, err := prometheus.New()
	if err != nil {
		slog.Error("Failed to initialize Prometheus exporter, metrics will be disabled",
			slog.Any("error", err))
		return nil
	}

	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter.Reader))
	otel.SetMeterProvider(provider)

	http.Handle("/", promhttp.Handler())
	_port := fmt.Sprintf(":%d", port)

	go func() {
		if err := http.ListenAndServe(_port, nil); err != nil && err != http.ErrServerClosed {
			slog.Error("metrics HTTP server failed", slog.Any("error", err))
		}
	}()

	//TODO: fix
	//plog.Debug(fmt.Sprintf("Prometheus server running on %s", _port))

	return provider
}

// validate checks the configuration for invalid values and returns an error
// if any field is out of range.
func (c *Config) validate() error {
	if c.DeadLetterThrottleCount < 0 {
		return fmt.Errorf("DeadLetterThrottleCount must be >= 0, got %d", c.DeadLetterThrottleCount)
	}
	if c.DeadLetterThrottleInterval <= 0 {
		return fmt.Errorf("DeadLetterThrottleInterval must be > 0")
	}
	if c.LoggerFactory == nil {
		return fmt.Errorf("LoggerFactory must not be nil")
	}
	if c.StopTimeout <= 0 {
		return fmt.Errorf("StopTimeout must be > 0, got %v", c.StopTimeout)
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("RequestTimeout must be > 0, got %v", c.RequestTimeout)
	}
	return nil
}

// NewConfig returns a configuration with default values.
func NewConfig() *Config {
	return defaultConfig()
}
