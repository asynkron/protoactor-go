package actor

import (
	"fmt"
	"net/http"
	"time"

	"github.com/asynkron/protoactor-go/log"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregation"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
)

type Config struct {
	DeadLetterThrottleInterval  time.Duration      // throttle deadletter logging after this interval
	DeadLetterThrottleCount     int32              // throttle deadletter logging after this count
	DeadLetterRequestLogging    bool               // do not log dead-letters with sender
	DeveloperSupervisionLogging bool               // console log and promote supervision logs to Warning level
	DiagnosticsSerializer       func(Actor) string // extract diagnostics from actor and return as string
	MetricsProvider             metric.MeterProvider
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
	}
}

func defaultPrometheusProvider(port int) metric.MeterProvider {
	config := prometheus.Config{}
	c := controller.New(
		processor.NewFactory(
			selector.NewWithInexpensiveDistribution(),
			aggregation.CumulativeTemporalitySelector(),
			processor.WithMemory(true),
		),
	)

	exporter, err := prometheus.New(config, c)
	if err != nil {
		err = fmt.Errorf("failed to initialize prometheus exporter: %w", err)
		plog.Error(err.Error(), log.Error(err))

		return nil
	}

	provider := exporter.MeterProvider()
	global.SetMeterProvider(provider)

	http.HandleFunc("/", exporter.ServeHTTP)

	_port := fmt.Sprintf(":%d", port)

	go func() {
		_ = http.ListenAndServe(_port, nil)
	}()

	plog.Debug(fmt.Sprintf("Prometheus server running on %s", _port))

	return provider
}

func NewConfig() *Config {
	return defaultConfig()
}
