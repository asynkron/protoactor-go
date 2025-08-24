// Copyright (C) 2017 - 2024 Asynkron.se <http://www.asynkron.se>

package metrics

import (
	"fmt"
	"log/slog"
	"sync"
)

// InternalActorMetrics is the key under which builtin actor metrics are registered.
const InternalActorMetrics string = "internal.actor.metrics"

// ProtoMetrics manages metric instruments for actors.
type ProtoMetrics struct {
	mu           sync.Mutex
	actorMetrics *ActorMetrics
	knownMetrics map[string]*ActorMetrics
	logger       *slog.Logger
}

// NewProtoMetrics constructs a ProtoMetrics instance and registers default instruments.
func NewProtoMetrics(logger *slog.Logger) *ProtoMetrics {
	protoMetrics := ProtoMetrics{
		actorMetrics: NewActorMetrics(logger),
		knownMetrics: make(map[string]*ActorMetrics),
		logger:       logger,
	}

	protoMetrics.Register(InternalActorMetrics, protoMetrics.actorMetrics)
	return &protoMetrics
}

// Instruments returns the default ActorMetrics instance.
func (pm *ProtoMetrics) Instruments() *ActorMetrics { return pm.actorMetrics }

// Register associates the provided metrics instance with the given key.
func (pm *ProtoMetrics) Register(key string, instance *ActorMetrics) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	logger := pm.logger

	if _, ok := pm.knownMetrics[key]; ok {
		err := fmt.Errorf("could not register instance %#v of metrics, %s already registered", instance, key)
		logger.Error(err.Error(), slog.Any("error", err))
		return
	}

	pm.knownMetrics[key] = instance
}

// Get retrieves the metrics instance registered under key, or nil if none exist.
func (pm *ProtoMetrics) Get(key string) *ActorMetrics {
	metrics, ok := pm.knownMetrics[key]
	if !ok {
		logger := pm.logger
		err := fmt.Errorf("unknown metrics for the given %s key", key)
		logger.Error(err.Error(), slog.Any("error", err))
		return nil
	}

	return metrics
}
