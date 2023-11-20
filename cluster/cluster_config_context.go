// Copyright (C) 2017 - 2022 Asynkron.se <http://www.asynkron.se>

package cluster

import (
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

const (
	defaultActorRequestTimeout                             = 5 * time.Second
	defaultRequestsLogThrottlePeriod                       = 2 * time.Second
	defaultMaxNumberOfEvetsInRequestLogThrottledPeriod int = 3
)

// ClusterContextConfig is used to configure cluster context parameters
type ClusterContextConfig struct {
	RequestsLogThrottlePeriod                    time.Duration
	MaxNumberOfEventsInRequestLogThrottledPeriod int
	requestLogThrottle                           actor.ShouldThrottle
}
