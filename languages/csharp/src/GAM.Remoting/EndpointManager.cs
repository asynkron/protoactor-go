// -----------------------------------------------------------------------
//  <copyright file="EndpointManager.cs" company="Asynkron HB">
//      Copyright (C) 2015-2016 Asynkron HB All rights reserved
//  </copyright>
// -----------------------------------------------------------------------

using System;
using System.Collections.Generic;
using System.Threading.Tasks;

namespace GAM.Remoting
{
    public class EndpointManager : IActor
    {
        private readonly Dictionary<string, PID> _connections = new Dictionary<string, PID>();

        public Task ReceiveAsync(IContext context)
        {
            var msg = context.Message;
            if (msg is Started)
            {
                Console.WriteLine("[REMOTING] Started EndpointManager");
                return Actor.Done;
            }
            if (msg is MessageEnvelope)
            {
                var env = (MessageEnvelope) msg;
                PID pid;
                if (!_connections.TryGetValue(env.Target.Host, out pid))
                {
                    var props = Actor.FromProducer(() => new EndpointWriter(env.Target.Host));
                    pid = context.Spawn(props);
                    _connections.Add(env.Target.Host, pid);
                }
                pid.Tell(msg);
                return Actor.Done;
            }
            return Actor.Done;
        }
    }
}