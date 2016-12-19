// -----------------------------------------------------------------------
//  <copyright file="RemoteActorRef.cs" company="Asynkron HB">
//      Copyright (C) 2015-2016 Asynkron HB All rights reserved
//  </copyright>
// -----------------------------------------------------------------------

using System;
using Google.Protobuf;

namespace GAM.Remoting
{
    public class RemoteActorRef : ActorRef
    {
        private readonly PID _pid;

        public RemoteActorRef(PID pid)
        {
            _pid = pid;
        }

        public override void SendUserMessage(PID pid, object message, PID sender)
        {
            Send(pid, message, sender);
        }

        public override void SendSystemMessage(PID pid, SystemMessage sys)
        {
            Send(pid, sys, null);
        }

        private void Send(PID pid, object msg, PID sender)
        {
            if (msg is IMessage)
            {
                var imsg = (IMessage) msg;
                var env = new MessageEnvelope
                {
                    Target = _pid,
                    Sender = sender,
                    MessageData = Serialization.Serialize(imsg),
                    TypeName = imsg.Descriptor.File.Package + "." + imsg.Descriptor.Name
                };
                RemotingSystem.EndpointManagerPid.Tell(env);
            }
            else
            {
                throw new NotSupportedException("Non protobuf message");
            }
        }
    }
}