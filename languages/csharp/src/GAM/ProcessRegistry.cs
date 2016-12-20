// -----------------------------------------------------------------------
//  <copyright file="ProcessRegistry.cs" company="Asynkron HB">
//      Copyright (C) 2015-2016 Asynkron HB All rights reserved
//  </copyright>
// -----------------------------------------------------------------------

using System;
using System.Collections.Generic;
using System.Threading;

namespace GAM
{
    public class ProcessRegistry
    {
        private readonly IList<Func<PID, ActorRef>> _hostResolvers = new List<Func<PID, ActorRef>>();

        private readonly HashedConcurrentDictionary _localActorRefs =
            new HashedConcurrentDictionary();

        private int _sequenceId;
        public static ProcessRegistry Instance { get; } = new ProcessRegistry();

        public string Host { get; set; } = "nonhost";

        public void RegisterHostResolver(Func<PID, ActorRef> resolver)
        {
            _hostResolvers.Add(resolver);
        }

        public ActorRef Get(PID pid)
        {
            if (pid.Host != "nonhost" && pid.Host != Host)
            {
                foreach (var resolver in _hostResolvers)
                {
                    var reff = resolver(pid);
                    if (reff != null)
                    {
                        return reff;
                    }
                }
                throw new NotSupportedException("Unknown host");
            }

            ActorRef aref;
            if (_localActorRefs.TryGetValue(pid.Id, out aref))
            {
                return aref;
            }
            return DeadLetterActorRef.Instance;
        }

        public ValueTuple<PID, bool> TryAdd(string id, ActorRef aref)
        {
            var pid = new PID()
            {
                Id = id,
                Ref = aref, //cache aref lookup
                Host = "nonhost" // local
            };
            var ok = _localActorRefs.TryAdd(pid.Id, aref);
            return ValueTuple.Create(pid, ok);
        }

        public void Remove(PID pid)
        {
            _localActorRefs.Remove(pid.Id);
        }

        internal string GetAutoId()
        {
            var counter = Interlocked.Increment(ref _sequenceId);
            return "$" + counter;
        }
    }
}