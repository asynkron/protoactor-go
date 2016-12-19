using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using Google.Protobuf;

namespace GAM.Remoting
{
    public class RemoteActorRef : ActorRef
    {
        public override void SendUserMessage(PID pid, object message,PID sender)
        {
            Send(pid,message,sender);
        }

        public override void SendSystemMessage(PID pid, SystemMessage sys)
        {
            Send(pid,sys,null);
        }

        private void Send(PID pid, object msg, PID sender)
        {
            if (msg is IMessage)
            {
                var imsg = (IMessage) msg;
                var env = new MessageEnvelope
                {
                    Target = pid,
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
