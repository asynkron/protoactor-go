// -----------------------------------------------------------------------
//  <copyright file="DeadLetter.cs" company="Asynkron HB">
//      Copyright (C) 2015-2016 Asynkron HB All rights reserved
//  </copyright>
// -----------------------------------------------------------------------

namespace GAM
{
    public class DeadLetter
    {
        public DeadLetter(PID pid, object message)
        {
            Pid = pid;
            Message = message;
        }

        public PID Pid { get; }
        public object Message { get; }
    }

    public class DeadLetterActorRef : ActorRef
    {
        public static readonly DeadLetterActorRef Instance = new DeadLetterActorRef();

        public override void SendUserMessage(PID pid, object message, PID sender)
        {
            EventStream.Instance.Publish(new DeadLetter(pid, message));
        }

        public override void SendSystemMessage(PID pid, SystemMessage sys)
        {
            EventStream.Instance.Publish(new DeadLetter(pid, sys));
        }
    }
}