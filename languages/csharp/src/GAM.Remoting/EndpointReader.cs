using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using Grpc.Core;
using Grpc.Core.Utils;

namespace GAM.Remoting
{
    public class EndpointReader : Remoting.RemotingBase
    {
        public override async Task<Unit> Receive(IAsyncStreamReader<MessageBatch> requestStream, ServerCallContext context)
        {
            await requestStream.ForEachAsync(batch =>
            {
                foreach (var envelope in batch.Envelopes)
                {
                    Console.WriteLine(envelope.ToString());
                }
               
                return Actor.Done;
            });
            return new Unit();
        }
    }
}
