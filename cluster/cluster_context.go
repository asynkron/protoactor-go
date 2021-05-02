package cluster

import "fmt"

type ClusterContext interface {
	Request(identity string, kind string, message interface{}) (interface{}, error)
}

func NewDefaultClusterContext(cluster *Cluster) ClusterContext {
	return &DefaultClusterContext{
		cluster: cluster,
	}
}

type DefaultClusterContext struct {
	cluster *Cluster
}

func (d DefaultClusterContext) Request(identity string, kind string, message interface{}) (interface{}, error) {
	return nil, fmt.Errorf("foo")
}

/*
// -----------------------------------------------------------------------
// <copyright file="DefaultClusterContext.cs" company="Asynkron AB">
//      Copyright (C) 2015-2020 Asynkron AB All rights reserved
// </copyright>
// -----------------------------------------------------------------------
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using Proto.Cluster.Identity;
using Proto.Cluster.Metrics;
using Proto.Future;
using Proto.Utils;

namespace Proto.Cluster
{
    public class DefaultClusterContext : IClusterContext
    {
        private readonly IIdentityLookup _identityLookup;

        private readonly PidCache _pidCache;
        private readonly ShouldThrottle _requestLogThrottle;
        private readonly TaskClock _clock;
        private static readonly ILogger Logger = Log.CreateLogger<DefaultClusterContext>();

        public DefaultClusterContext(IIdentityLookup identityLookup, PidCache pidCache, ClusterContextConfig config, CancellationToken killSwitch)
        {
            _identityLookup = identityLookup;
            _pidCache = pidCache;

            _requestLogThrottle = Throttle.Create(
                config.MaxNumberOfEventsInRequestLogThrottlePeriod,
                config.RequestLogThrottlePeriod,
                i => Logger.LogInformation("Throttled {LogCount} TryRequestAsync logs", i)
            );
            _clock = new TaskClock(config.ActorRequestTimeout, TimeSpan.FromSeconds(1), killSwitch);
            _clock.StartMember();
        }

        public async Task<T?> RequestAsync<T>(ClusterIdentity clusterIdentity, object message, ISenderContext context, CancellationToken ct)
        {
            var start = DateTime.UtcNow;
            Logger.LogDebug("Requesting {ClusterIdentity} Message {Message}", clusterIdentity, message);
            var i = 0;

            var future = new FutureProcess(context.System);
            PID? lastPid = null;

            try
            {
                while (!ct.IsCancellationRequested)
                {
                    if (context.System.Shutdown.IsCancellationRequested) return default;

                    var delay = i * 20;
                    i++;

                    var (pid, source) = await GetPid(clusterIdentity, context, ct);

                    if (context.System.Shutdown.IsCancellationRequested) return default;

                    if (pid is null)
                    {
                        Logger.LogDebug("Requesting {ClusterIdentity} - Did not get PID from IdentityLookup", clusterIdentity);
                        await Task.Delay(delay, CancellationToken.None);
                        continue;
                    }

                    // Ensures that a future is not re-used against another actor.
                    if (lastPid is not null && !pid.Equals(lastPid)) RefreshFuture();

                    Logger.LogDebug("Requesting {ClusterIdentity} - Got PID {Pid} from {Source}", clusterIdentity, pid, source);
                    var (status, res) = await TryRequestAsync<T>(clusterIdentity, message, pid, source, context, future);

                    switch (status)
                    {
                        case ResponseStatus.Ok:
                            return res;

                        case ResponseStatus.Exception:
                            RefreshFuture();
                            await RemoveFromSource(clusterIdentity, PidSource.Cache, pid);
                            await Task.Delay(delay, CancellationToken.None);
                            break;
                        case ResponseStatus.DeadLetter:
                            RefreshFuture();
                            await RemoveFromSource(clusterIdentity, source, pid);
                            break;
                        case ResponseStatus.TimedOut:
                            lastPid = pid;
                            await RemoveFromSource(clusterIdentity, PidSource.Cache, pid);
                            break;
                    }

                    if (!context.System.Metrics.IsNoop)
                    {
                        context.System.Metrics.Get<ClusterMetrics>().ClusterRequestRetryCount.Inc(new[]
                            {context.System.Id, context.System.Address, clusterIdentity.Kind, message.GetType().Name}
                        );
                    }
                }

                if (!context.System.Shutdown.IsCancellationRequested && _requestLogThrottle().IsOpen())
                {
                    var t = DateTime.UtcNow - start;
                    Logger.LogWarning("RequestAsync retried but failed for {ClusterIdentity}, elapsed {Time}", clusterIdentity, t);
                }

                return default!;
            }
            finally
            {
                future.Dispose();
            }

            void RefreshFuture()
            {
                future.Dispose();
                future = new FutureProcess(context.System);
                lastPid = null;
            }
        }

        private async Task RemoveFromSource(ClusterIdentity clusterIdentity, PidSource source, PID pid)
        {
            if (source == PidSource.IdentityLookup) await _identityLookup.RemovePidAsync(clusterIdentity, pid, CancellationToken.None);

            _pidCache.RemoveByVal(clusterIdentity, pid);
        }

        private async ValueTask<(PID?, PidSource)> GetPid(ClusterIdentity clusterIdentity, ISenderContext context, CancellationToken ct)
        {
            try
            {
                if (_pidCache.TryGet(clusterIdentity, out var cachedPid)) return (cachedPid, PidSource.Cache);

                if (!context.System.Metrics.IsNoop)
                {
                    var pid = await context.System.Metrics.Get<ClusterMetrics>().ClusterResolvePidHistogram
                        .Observe(async () => await _identityLookup.GetAsync(clusterIdentity, ct), context.System.Id, context.System.Address,
                            clusterIdentity.Kind
                        );

                    if (pid is not null) _pidCache.TryAdd(clusterIdentity, pid);
                    return (pid, PidSource.IdentityLookup);
                }
                else
                {
                    var pid = await _identityLookup.GetAsync(clusterIdentity, ct);
                    if (pid is not null) _pidCache.TryAdd(clusterIdentity, pid);
                    return (pid, PidSource.IdentityLookup);
                }
            }
            catch (Exception e)
            {
                if (context.System.Shutdown.IsCancellationRequested) return default;

                if (_requestLogThrottle().IsOpen())
                    Logger.LogWarning(e, "Failed to get PID from IIdentityLookup for {ClusterIdentity}", clusterIdentity);
                return (null, PidSource.IdentityLookup);
            }
        }

        private async ValueTask<(ResponseStatus Ok, T?)> TryRequestAsync<T>(
            ClusterIdentity clusterIdentity,
            object message,
            PID pid,
            PidSource source,
            ISenderContext context,
            FutureProcess future
        )
        {
            var t = DateTimeOffset.UtcNow;

            try
            {
                context.Request(pid, message, future.Pid);
                var task = future.GetTask();
                await Task.WhenAny(task, _clock.CurrentBucket);

                if (task.IsCompleted)
                {
                    var res = task.Result;

                    return ToResult<T>(source, context, res);
                }

                if (!context.System.Shutdown.IsCancellationRequested)
                    Logger.LogDebug("TryRequestAsync timed out, PID from {Source}", source);
                _pidCache.RemoveByVal(clusterIdentity, pid);

                return (ResponseStatus.TimedOut, default)!;
            }
            catch (TimeoutException)
            {
                return (ResponseStatus.TimedOut, default)!;
            }
            catch (Exception x)
            {
                if (!context.System.Shutdown.IsCancellationRequested && _requestLogThrottle().IsOpen())
                    Logger.LogDebug(x, "TryRequestAsync failed with exception, PID from {Source}", source);
                _pidCache.RemoveByVal(clusterIdentity, pid);
                return (ResponseStatus.Exception, default)!;
            }
            finally
            {
                if (!context.System.Metrics.IsNoop)
                {
                    var elapsed = DateTimeOffset.UtcNow - t;
                    context.System.Metrics.Get<ClusterMetrics>().ClusterRequestHistogram
                        .Observe(elapsed, new[]
                            {
                                context.System.Id, context.System.Address, clusterIdentity.Kind, message.GetType().Name,
                                source == PidSource.Cache ? "PidCache" : "IIdentityLookup"
                            }
                        );
                }
            }
        }

        private (ResponseStatus Ok, T?) ToResult<T>(PidSource source, ISenderContext context, object result)
        {
            switch (result)
            {
                case DeadLetterResponse:
                    if (!context.System.Shutdown.IsCancellationRequested)
                        Logger.LogDebug("TryRequestAsync failed, dead PID from {Source}", source);

                    return (ResponseStatus.DeadLetter, default)!;
                case null: return (ResponseStatus.Ok, default);
                case T t:  return (ResponseStatus.Ok, t);
                default:
                    Logger.LogWarning("Unexpected message. Was type {Type} but expected {ExpectedType}", result.GetType(), typeof(T));
                    return (ResponseStatus.Exception, default);
            }
        }

        private enum ResponseStatus
        {
            Ok,
            TimedOut,
            Exception,
            DeadLetter
        }

        private enum PidSource
        {
            Cache,
            IdentityLookup
        }
    }
}
*/
