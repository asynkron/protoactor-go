// //-----------------------------------------------------------------------
// // <copyright file="ProcessRegistry.cs" company="Asynkron HB">
// //     Copyright (C) 2015-2016 Asynkron HB All rights reserved
// // </copyright>
// //-----------------------------------------------------------------------

using System.Collections.Concurrent;

namespace GAM.Actor
{
    public class ProcessRegistry
    {
        private readonly ConcurrentDictionary<PID, ActorRef> _localActorRefs = new ConcurrentDictionary<PID, ActorRef>();
        public static ProcessRegistry Instance { get; } = new ProcessRegistry();

        public ActorRef Get(PID pid)
        {
            ActorRef aref;
            if (_localActorRefs.TryGetValue(pid, out aref))
            {
                return aref;
            }
            return null;
        }

        public bool Add(PID pid, ActorRef aref)
        {
            return _localActorRefs.TryAdd(pid, aref);
        }
    }
}