using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace GAM.Actor
{
    public class PID
    {
        
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
