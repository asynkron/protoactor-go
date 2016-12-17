// //-----------------------------------------------------------------------
// // <copyright file="ActorRef.cs" company="Asynkron HB">
// //     Copyright (C) 2015-2016 Asynkron HB All rights reserved
// // </copyright>
// //-----------------------------------------------------------------------

using System.Threading.Tasks;

namespace GAM
{
    public class PID
    {
        public void Tell(object message)
        {
            var reff = ProcessRegistry.Instance.Get(this);
            reff.Tell(message);
        }
        public string Id { get; set; }

        public void SendSystemMessage(SuspendMailbox suspendMailbox)
        {
            var reff = ProcessRegistry.Instance.Get(this);
            //TODO: send system message
        }
    }

    public class Request
    {
        public Request(object message, PID sender)
        {
            Message = message;
            Sender = sender;
        }

        public object Message { get; }
        public PID Sender { get; }
    }

    public abstract class ActorRef
    {
        public abstract void Tell(object message);
    }

    public static class ActorRefExtensions
    {
        public static void Request(this ActorRef self, object message, PID sender)
        {
            self.Tell(new Request(message, sender));
        }

        public static Task<T> RequestAsync<T>(this ActorRef self, object message, PID sender)
        {
            return null;
        }
    }
}