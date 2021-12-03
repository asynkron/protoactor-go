// Copyright (C) 2017 - 2022 Asynkron.se <http://www.asynkron.se>

package cluster

import (
	"fmt"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
)

const (
	defaultActorRequestTimeout                         time.Duration = 5 * time.Second
	defaultRequestsLogThrottlePeriod                   time.Duration = 2 * time.Second
	defaultMaxNumberOfEvetsInRequestLogThrottledPeriod int           = 3
)

// Data structure used to configure cluster context parameters
type ClusterContextConfig struct {
	ActorRequestTimeout                          time.Duration
	RequestsLogThrottlePeriod                    time.Duration
	MaxNumberOfEventsInRequestLogThrottledPeriod int
	RetryAction                                  func(int) int
	requestLogThrottle                           actor.ShouldThrottle
}

// Creates a mew ClusterContextConfig with default
// values and returns a pointer to its memory address
func NewDefaultClusterContextConfig() *ClusterContextConfig {

	config := ClusterContextConfig{
		ActorRequestTimeout:                          defaultActorRequestTimeout,
		RequestsLogThrottlePeriod:                    defaultRequestsLogThrottlePeriod,
		MaxNumberOfEventsInRequestLogThrottledPeriod: defaultMaxNumberOfEvetsInRequestLogThrottledPeriod,
		RetryAction: defaultRetryAction,
		requestLogThrottle: actor.NewThrottle(
			int32(defaultMaxNumberOfEvetsInRequestLogThrottledPeriod),
			defaultRequestsLogThrottlePeriod,
			func(i int32) {
				plog.Info(fmt.Sprintf("Throttled %d Request logs", i))
			},
		),
	}
	return &config
}
