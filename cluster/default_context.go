// Copyright (C) 2017 - 2024 Asynkron.se <http://www.asynkron.se>

package cluster

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sync"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/remote"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Defines a type to provide DefaultContext configurations / implementations.
type ContextProducer func(*Cluster) Context

// selfCachingLookup is an optional interface that an IdentityLookup may
// implement to signal that it manages its own PidCache.Set calls. When true,
// DefaultContext must not call PidCache.Set on the result of Get() — doing
// so would bypass the lookup's own caching policy (e.g. the natskv grace
// window that intentionally withholds caching for absent-owner activations).
type selfCachingLookup interface {
	SetsOwnPidCache() bool
}

// Defines a default cluster context hashBytes structure.
type DefaultContext struct {
	cluster                   *Cluster
	requestTimeoutLogThrottle actor.ShouldThrottle
	futureTimeoutLogThrottle  actor.ShouldThrottle
	// skipPidCacheSet and skipPidCacheSetOnce implement a lazy one-time check
	// of whether the cluster's IdentityLookup implements selfCachingLookup.
	// The check is deferred until the first getPid call because IdentityLookup
	// is set on the Cluster by StartMember/StartClient, which happens after
	// newDefaultClusterContext is called during cluster construction.
	skipPidCacheSet     bool
	skipPidCacheSetOnce sync.Once
}

var _ Context = (*DefaultContext)(nil)

// Creates a new DefaultContext value and returns
// a pointer to its memory address as a Context.
func newDefaultClusterContext(cluster *Cluster) Context {
	clusterContext := DefaultContext{
		cluster: cluster,
		requestTimeoutLogThrottle: actor.NewThrottle(5, 10*time.Second, func(count int32) {
			cluster.Logger().Warn("Cluster request timeout logging throttled",
				slog.Int("suppressed", int(count)))
		}),
		futureTimeoutLogThrottle: actor.NewThrottle(5, 10*time.Second, func(count int32) {
			cluster.Logger().Warn("Cluster future request timeout logging throttled",
				slog.Int("suppressed", int(count)))
		}),
	}

	return &clusterContext
}

// selfCachingSkip resolves (lazily, once) whether the cluster's IdentityLookup
// sets its own PidCache entries and DefaultContext should skip its own Set calls.
func (dcc *DefaultContext) selfCachingSkip() bool {
	dcc.skipPidCacheSetOnce.Do(func() {
		if s, ok := dcc.cluster.IdentityLookup.(selfCachingLookup); ok && s.SetsOwnPidCache() {
			dcc.skipPidCacheSet = true
		}
	})
	return dcc.skipPidCacheSet
}

func (dcc *DefaultContext) Request(identity, kind string, message any, opts ...GrainCallOption) (any, error) {
	var err error

	var resp any

	var counter int
	callConfig := NewGrainCallOptions(dcc.cluster)
	for _, o := range opts {
		o(callConfig)
	}

	if len(callConfig.Headers) > 0 {
		message = actor.EnvelopeWithHeaders(message, callConfig.Headers)
	}

	_context := callConfig.Context

	// get the configuration from the composed Cluster value
	cfg := dcc.cluster.Config.ToClusterContextConfig(dcc.cluster.Logger())

	start := time.Now()

	traceCtx, span := otel.Tracer("protoactor/cluster").Start(context.Background(), "cluster.request",
		trace.WithAttributes(
			attribute.String("kind", kind),
			attribute.String("identity", identity),
			attribute.String("messagetype", reflect.TypeOf(message).String()),
		),
	)
	defer span.End()

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
			err = fmt.Errorf("request failed: %w", ctx.Err())
			// Log on first timeout signal, before retry loop exits.
			// A separate throttle (cfg.requestLogThrottle) fires after
			// all retries are exhausted, logging total duration.
			if dcc.requestTimeoutLogThrottle() == actor.Open {
				dcc.cluster.Logger().Warn("Cluster request timed out",
					slog.String("identity", identity),
					slog.String("kind", kind))
			}
			break selectloop
		default:
			if counter >= callConfig.RetryCount {
				if err != nil { // a dead-letter/timeout sentinel caused the exhaustion
					err = fmt.Errorf("cluster request to %s/%s reached max retries (%d): %w: %w",
						kind, identity, callConfig.RetryCount, ErrMaxRetriesExceeded, err)
				} else { // exhausted on pid-nil resolution failures; no inner sentinel
					err = fmt.Errorf("cluster request to %s/%s reached max retries (%d): %w",
						kind, identity, callConfig.RetryCount, ErrMaxRetriesExceeded)
				}
				break selectloop
			}
			pid, fromCache = dcc.getPid(traceCtx, identity, kind)
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

			// RequestFuture.Result() can return both a response and an error when the
			// target actor processes the message but the response delivery encounters
			// an issue (e.g., DeadLetterResponse). When we have a valid response,
			// we use it regardless of the error, since the actor did produce a result.
			resp, err = _context.RequestFuture(pid, message, ttl).Result()
			if resp != nil {
				if err != nil {
					dcc.cluster.Logger().Debug("Cluster request returned both response and error",
						slog.String("identity", identity),
						slog.String("kind", kind),
						slog.Any("error", err))
				}
				break selectloop
			}
			if err != nil {
				dcc.cluster.Logger().Error("cluster.RequestFuture failed", slog.Any("error", err), slog.Any("pid", pid))

				isDeadLetter := errors.Is(err, actor.ErrDeadLetter) || errors.Is(err, remote.ErrDeadLetter)
				isTimeout := errors.Is(err, actor.ErrTimeout) || errors.Is(err, remote.ErrTimeout)

				if isDeadLetter || isTimeout {
					counter = callConfig.RetryAction(counter)
					dcc.cluster.PidCache.Remove(identity, kind)

					// Only call RemovePid on dead letter — the actor process
					// is confirmed gone. Timeouts may indicate a slow-but-alive
					// actor; deleting its identity record would orphan the
					// process (alive locally but unresolvable via identity lookup).
					if isDeadLetter {
						dcc.cluster.IdentityLookup.RemovePid(
							NewClusterIdentity(identity, kind), pid,
						)
					}
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
				break selectloop
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

	if dcc.cluster.metricsEnabled && err == nil {
		_ctx := context.Background()
		dcc.cluster.metrics.ClusterMessageSentCount.Add(_ctx, 1, metric.WithAttributes(attribute.String("kind", kind)))
	}

	if contextError := ctx.Err(); contextError != nil && cfg.requestLogThrottle() == actor.Open {
		// context timeout exceeded, report and return
		dcc.cluster.Logger().Warn("Request retried but failed", slog.String("identity", identity), slog.String("kind", kind), slog.Duration("duration", totalTime))
	}

	return resp, err
}

func (dcc *DefaultContext) RequestFuture(identity string, kind string, message any, opts ...GrainCallOption) (actor.Future, error) {
	var counter int
	callConfig := NewGrainCallOptions(dcc.cluster)
	for _, o := range opts {
		o(callConfig)
	}

	if len(callConfig.Headers) > 0 {
		message = actor.EnvelopeWithHeaders(message, callConfig.Headers)
	}

	_context := callConfig.Context

	traceCtx, span := otel.Tracer("protoactor/cluster").Start(context.Background(), "cluster.request",
		trace.WithAttributes(
			attribute.String("kind", kind),
			attribute.String("identity", identity),
			attribute.String("messagetype", reflect.TypeOf(message).String()),
		),
	)
	defer span.End()

	dcc.cluster.Logger().Debug("Requesting future", slog.String("identity", identity), slog.String("kind", kind), slog.String("type", reflect.TypeOf(message).String()), slog.Any("message", message))

	// crate a new Timeout Context
	ttl := callConfig.Timeout

	ctx, cancel := context.WithTimeout(context.Background(), ttl)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			err := fmt.Errorf("request failed: %w", ctx.Err())
			if dcc.futureTimeoutLogThrottle() == actor.Open {
				dcc.cluster.Logger().Warn("Cluster future request timed out",
					slog.String("identity", identity),
					slog.String("kind", kind))
			}
			return nil, err
		default:
			if counter >= callConfig.RetryCount {
				return nil, fmt.Errorf("cluster request to %s/%s reached max retries (%d): %w",
					kind, identity, callConfig.RetryCount, ErrMaxRetriesExceeded)
			}

			pid, _ := dcc.getPid(traceCtx, identity, kind)
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
func (dcc *DefaultContext) getPid(traceCtx context.Context, identity, kind string) (*actor.PID, bool) {
	if pid, ok := dcc.cluster.PidCache.Get(identity, kind); ok {
		if dcc.cluster.metricsEnabled {
			_ctx := context.Background()
			dcc.cluster.metrics.IdentityCacheHitCount.Add(_ctx, 1, metric.WithAttributes(actor.SystemLabels(dcc.cluster.ActorSystem)...))
		}
		return pid, true
	}

	_, resolveSpan := otel.Tracer("protoactor/cluster").Start(traceCtx, "cluster.resolve_pid",
		trace.WithAttributes(
			attribute.String("kind", kind),
			attribute.String("identity", identity),
		),
	)
	defer resolveSpan.End()

	if dcc.cluster.metricsEnabled {
		_ctx := context.Background()
		dcc.cluster.metrics.IdentityCacheMissCount.Add(_ctx, 1, metric.WithAttributes(actor.SystemLabels(dcc.cluster.ActorSystem)...))
	}

	var pid *actor.PID
	if dcc.cluster.metricsEnabled {
		start := time.Now()
		pid = dcc.cluster.Get(identity, kind)
		if pid != nil && !dcc.selfCachingSkip() {
			dcc.cluster.PidCache.Set(identity, kind, pid)
		}
		elapsed := time.Since(start)
		_ctx := context.Background()
		attrs := append(
			actor.SystemLabels(dcc.cluster.ActorSystem),
			attribute.String("clusterkind", kind),
		)
		dcc.cluster.metrics.ClusterResolvePidDuration.Record(_ctx, elapsed.Seconds(), metric.WithAttributes(attrs...))
		dcc.cluster.metrics.IdentityLookupDuration.Record(_ctx, elapsed.Seconds(), metric.WithAttributes(attribute.String("kind", kind)))
		if pid == nil {
			dcc.cluster.metrics.IdentityLookupFailureCount.Add(_ctx, 1, metric.WithAttributes(attribute.String("kind", kind)))
		}
	} else {
		pid = dcc.cluster.Get(identity, kind)
		if pid != nil && !dcc.selfCachingSkip() {
			dcc.cluster.PidCache.Set(identity, kind, pid)
		}
	}

	return pid, false
}
