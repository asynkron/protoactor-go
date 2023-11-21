// Copyright (C) 2017 - 2022 Asynkron.se <http://www.asynkron.se>

package metrics

import (
	"fmt"
	"log/slog"
	"sync"
)

const InternalActorMetrics string = "internal.actor.metrics"

type ProtoMetrics struct {
	mu           sync.Mutex
	actorMetrics *ActorMetrics
	knownMetrics map[string]*ActorMetrics
	logger       *slog.Logger
}

func NewProtoMetrics(logger *slog.Logger) *ProtoMetrics {
	protoMetrics := ProtoMetrics{
		actorMetrics: NewActorMetrics(logger),
		knownMetrics: make(map[string]*ActorMetrics),
		logger:       logger,
	}

	protoMetrics.Register(InternalActorMetrics, protoMetrics.actorMetrics)
	return &protoMetrics
}

func (pm *ProtoMetrics) Instruments() *ActorMetrics { return pm.actorMetrics }

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
