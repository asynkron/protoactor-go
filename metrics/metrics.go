// Copyright (C) 2017 - 2022 Asynkron.se <http://www.asynkron.se>

package metrics

import (
	"fmt"
	"sync"

	"github.com/asynkron/protoactor-go/log"
	"go.opentelemetry.io/otel/metric"
)

const InternalActorMetrics string = "internal.actor.metrics"

type ProtoMetrics struct {
	mu           sync.Mutex
	actorMetrics *ActorMetrics
	knownMetrics map[string]*ActorMetrics
}

func NewProtoMetrics(provider metric.MeterProvider) *ProtoMetrics {
	protoMetrics := ProtoMetrics{
		actorMetrics: NewActorMetrics(),
		knownMetrics: make(map[string]*ActorMetrics),
	}

	protoMetrics.Register(InternalActorMetrics, protoMetrics.actorMetrics)
	return &protoMetrics
}

func (pm *ProtoMetrics) Instruments() *ActorMetrics { return pm.actorMetrics }

func (pm *ProtoMetrics) Register(key string, instance *ActorMetrics) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.knownMetrics[key]; ok {
		err := fmt.Errorf("could not register instance %#v of metrics, %s already registered", instance, key)
		plog.Error(err.Error(), log.Error(err))
		return
	}

	pm.knownMetrics[key] = instance
}

func (pm *ProtoMetrics) Get(key string) *ActorMetrics {
	metrics, ok := pm.knownMetrics[key]
	if !ok {
		err := fmt.Errorf("unknown metrics for the given %s key", key)
		plog.Error(err.Error(), log.Error(err))
		return nil
	}

	return metrics
}
