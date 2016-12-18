using System;
using System.Threading.Tasks;
using GAM;
using GAM.Remoting;
using Messages;

namespace Node1
{
    public class EchoActor : IActor
    {
        private PID _sender;
     //   private int counter = 0;
        public Task ReceiveAsync(IContext context)
        {
            var msg = context.Message;
            if (msg is StartRemote)
            {
                var sr = (StartRemote) msg;
                Console.WriteLine("Starting");
                _sender = sr.Sender;
                context.Respond(new Start());
                return Actor.Done;
            }

            if (msg is Ping)
            {
              //  counter++;
                //if (counter%100000 == 0)
                //{
                //    Console.WriteLine(counter);
                //}
                _sender.Tell(new Pong());
                return Actor.Done;
            }
            return Actor.Done;
        }
    }
    class Program
    {
        static void Main(string[] args)
        {
            Serialization.RegisterFileDescriptor(Messages.ProtosReflection.Descriptor);
            RemotingSystem.Start("127.0.0.1",8080);
            var props = Actor.FromProducer(() => new EchoActor());
            var pid = Actor.SpawnNamed(props, "remote");
            Console.ReadLine();
        }
    }
}
