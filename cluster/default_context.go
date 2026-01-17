// Copyright (C) 2017 - 2024 Asynkron.se <http://www.asynkron.se>

package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Defines a type to provide DefaultContext configurations / implementations.
type ContextProducer func(*Cluster) Context

// Defines a default cluster context hashBytes structure.
type DefaultContext struct {
	cluster *Cluster
}

var _ Context = (*DefaultContext)(nil)

// Creates a new DefaultContext value and returns
// a pointer to its memory address as a Context.
func newDefaultClusterContext(cluster *Cluster) Context {
	clusterContext := DefaultContext{
		cluster: cluster,
	}

	return &clusterContext
}

func (dcc *DefaultContext) Request(identity, kind string, message interface{}, opts ...GrainCallOption) (interface{}, error) {
	var err error

	var resp interface{}

	var counter int
	callConfig := NewGrainCallOptions(dcc.cluster)
	for _, o := range opts {
		o(callConfig)
	}

	_context := callConfig.Context

	// get the configuration from the composed Cluster value
	cfg := dcc.cluster.Config.ToClusterContextConfig(dcc.cluster.Logger())

	start := time.Now()

	dcc.cluster.Logger().Debug("Requesting", slog.String("identity", identity), slog.String("kind", kind), slog.String("type", reflect.TypeOf(message).String()), slog.Any("message", message))

	// crate a new Timeout Context
	ttl := callConfig.Timeout

	ctx, cancel := context.WithTimeout(context.Background(), ttl)
	defer cancel()

	var fromCache bool
	var pid *actor.PID

selectloop:
	for {
		select {
		case <-ctx.Done():
			// TODO: handler throttling and messaging here
			err = fmt.Errorf("request failed: %w", ctx.Err())

			break selectloop
		default:
			if counter >= callConfig.RetryCount {
				err = fmt.Errorf("have reached max retries: %v", callConfig.RetryCount)

				break selectloop
			}
			pid, fromCache = dcc.getPid(identity, kind)
			if pid == nil {
				dcc.cluster.Logger().Debug("Requesting PID from IdentityLookup but got nil", slog.String("identity", identity), slog.String("kind", kind))
				counter = callConfig.RetryAction(counter)
				if dcc.cluster.metricsEnabled {
					_ctx := context.Background()
					attrs := append(
						actor.SystemLabels(dcc.cluster.ActorSystem),
						attribute.String("clusterkind", kind),
						attribute.String("messagetype", actor.MessageName(message)),
					)
					dcc.cluster.metrics.ClusterRequestRetryCount.Add(_ctx, 1, metric.WithAttributes(attrs...))
				}
				continue
			}

			// TODO: why is err != nil when res != nil?
			resp, err = _context.RequestFuture(pid, message, ttl).Result()
			if resp != nil {
				break selectloop
			}
			if err != nil {
				dcc.cluster.Logger().Error("cluster.RequestFuture failed", slog.Any("error", err), slog.Any("pid", pid))
				switch err {
				case actor.ErrTimeout, remote.ErrTimeout, actor.ErrDeadLetter, remote.ErrDeadLetter:
					counter = callConfig.RetryAction(counter)
					dcc.cluster.PidCache.Remove(identity, kind)
					if dcc.cluster.metricsEnabled {
						_ctx := context.Background()
						attrs := append(
							actor.SystemLabels(dcc.cluster.ActorSystem),
							attribute.String("clusterkind", kind),
							attribute.String("messagetype", actor.MessageName(message)),
						)
						dcc.cluster.metrics.ClusterRequestRetryCount.Add(_ctx, 1, metric.WithAttributes(attrs...))
					}
					continue
				default:
					break selectloop
				}
			}
		}
	}

	totalTime := time.Since(start)
	if dcc.cluster.metricsEnabled {
		_ctx := context.Background()
		source := "IIdentityLookup"
		if fromCache {
			source = "PidCache"
		}
		attrs := append(
			actor.SystemLabels(dcc.cluster.ActorSystem),
			attribute.String("clusterkind", kind),
			attribute.String("messagetype", actor.MessageName(message)),
			attribute.String("pidsource", source),
		)
		dcc.cluster.metrics.ClusterRequestDuration.Record(_ctx, totalTime.Seconds(), metric.WithAttributes(attrs...))
	}

	if contextError := ctx.Err(); contextError != nil && cfg.requestLogThrottle() == actor.Open {
		// context timeout exceeded, report and return
		dcc.cluster.Logger().Warn("Request retried but failed", slog.String("identity", identity), slog.String("kind", kind), slog.Duration("duration", totalTime))
	}

	return resp, err
}

func (dcc *DefaultContext) RequestFuture(identity string, kind string, message interface{}, opts ...GrainCallOption) (actor.Future, error) {
	var counter int
	callConfig := NewGrainCallOptions(dcc.cluster)
	for _, o := range opts {
		o(callConfig)
	}

	_context := callConfig.Context

	dcc.cluster.Logger().Debug("Requesting future", slog.String("identity", identity), slog.String("kind", kind), slog.String("type", reflect.TypeOf(message).String()), slog.Any("message", message))

	// crate a new Timeout Context
	ttl := callConfig.Timeout

	ctx, cancel := context.WithTimeout(context.Background(), ttl)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			// TODO: handler throttling and messaging here
			err := fmt.Errorf("request failed: %w", ctx.Err())
			return nil, err
		default:
			if counter >= callConfig.RetryCount {
				return nil, fmt.Errorf("have reached max retries: %v", callConfig.RetryCount)
			}

			pid, _ := dcc.getPid(identity, kind)
			if pid == nil {
				dcc.cluster.Logger().Debug("Requesting PID from IdentityLookup but got nil", slog.String("identity", identity), slog.String("kind", kind))
				counter = callConfig.RetryAction(counter)
				if dcc.cluster.metricsEnabled {
					_ctx := context.Background()
					attrs := append(
						actor.SystemLabels(dcc.cluster.ActorSystem),
						attribute.String("clusterkind", kind),
						attribute.String("messagetype", actor.MessageName(message)),
					)
					dcc.cluster.metrics.ClusterRequestRetryCount.Add(_ctx, 1, metric.WithAttributes(attrs...))
				}
				continue
			}

			f := _context.RequestFuture(pid, message, ttl)
			return f, nil
		}
	}
}

// gets the cached PID for the given identity
// it can return nil if none is found.
func (dcc *DefaultContext) getPid(identity, kind string) (*actor.PID, bool) {
	if pid, ok := dcc.cluster.PidCache.Get(identity, kind); ok {
		return pid, true
	}

	var pid *actor.PID
	if dcc.cluster.metricsEnabled {
		start := time.Now()
		pid = dcc.cluster.Get(identity, kind)
		if pid != nil {
			dcc.cluster.PidCache.Set(identity, kind, pid)
		}
		elapsed := time.Since(start)
		_ctx := context.Background()
		attrs := append(
			actor.SystemLabels(dcc.cluster.ActorSystem),
			attribute.String("clusterkind", kind),
		)
		dcc.cluster.metrics.ClusterResolvePidDuration.Record(_ctx, elapsed.Seconds(), metric.WithAttributes(attrs...))
	} else {
		pid = dcc.cluster.Get(identity, kind)
		if pid != nil {
			dcc.cluster.PidCache.Set(identity, kind, pid)
		}
	}

	return pid, false
}
