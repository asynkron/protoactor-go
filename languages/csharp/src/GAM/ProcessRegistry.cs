// -----------------------------------------------------------------------
//  <copyright file="ProcessRegistry.cs" company="Asynkron HB">
//      Copyright (C) 2015-2016 Asynkron HB All rights reserved
//  </copyright>
// -----------------------------------------------------------------------

using System;
using System.Collections.Concurrent;
using System.Threading;

namespace GAM
{
    public class ProcessRegistry
    {
        private readonly HashedConcurrentDictionary _localActorRefs =
            new HashedConcurrentDictionary();

        private int _sequenceID;
        public static ProcessRegistry Instance { get; } = new ProcessRegistry();

        public ActorRef Get(PID pid)
        {
            ActorRef aref;
            if (_localActorRefs.TryGetValue(pid.Id, out aref))
            {
                return aref;
            }
            return null;
        }

        public ValueTuple<PID, bool> TryAdd(string id, ActorRef aref)
        {
            var pid = new PID()
            {
                Id = id,
                Ref = aref, //cache aref lookup
            };
            var ok = _localActorRefs.TryAdd(pid.Id, aref);
            return ValueTuple.Create(pid, ok);
        }

        public void Remove(PID pid)
        {
            ActorRef aref;
            _localActorRefs.Remove(pid.Id);
        }

        internal string GetAutoId()
        {
            var counter = Interlocked.Increment(ref _sequenceID);
            return "$" + counter;
        }
    }
}