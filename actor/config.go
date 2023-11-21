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

type Config struct {
	DeadLetterThrottleInterval  time.Duration      // throttle deadletter logging after this interval
	DeadLetterThrottleCount     int32              // throttle deadletter logging after this count
	DeadLetterRequestLogging    bool               // do not log dead-letters with sender
	DeveloperSupervisionLogging bool               // console log and promote supervision logs to Warning level
	DiagnosticsSerializer       func(Actor) string // extract diagnostics from actor and return as string
	MetricsProvider             metric.MeterProvider
	LoggerFactory               func(system *ActorSystem) *slog.Logger
}

func defaultConfig() *Config {
	return &Config{
		MetricsProvider:             nil,
		DeadLetterThrottleInterval:  1 * time.Second,
		DeadLetterThrottleCount:     3,
		DeadLetterRequestLogging:    true,
		DeveloperSupervisionLogging: false,
		DiagnosticsSerializer: func(actor Actor) string {
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
	}
}

func defaultPrometheusProvider(port int) metric.MeterProvider {
	exporter, err := prometheus.New()
	if err != nil {
		err = fmt.Errorf("failed to initialize prometheus exporter: %w", err)
		//TODO: fix
		//plog.Error(err.Error(), log.Error(err))

		return nil
	}

	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter.Reader))
	otel.SetMeterProvider(provider)

	http.Handle("/", promhttp.Handler())
	_port := fmt.Sprintf(":%d", port)

	go func() {
		_ = http.ListenAndServe(_port, nil)
	}()

	//TODO: fix
	//plog.Debug(fmt.Sprintf("Prometheus server running on %s", _port))

	return provider
}

func NewConfig() *Config {
	return defaultConfig()
}
