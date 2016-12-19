// -----------------------------------------------------------------------
//  <copyright file="EndpointWriter.cs" company="Asynkron HB">
//      Copyright (C) 2015-2016 Asynkron HB All rights reserved
//  </copyright>
// -----------------------------------------------------------------------

using System.Collections.Generic;
using System.Threading.Tasks;
using Grpc.Core;

namespace GAM.Remoting
{
    public class EndpointWriter : IActor
    {
        private readonly string _host;
        private Channel _channel;
        private Remoting.RemotingClient _client;
        private AsyncClientStreamingCall<MessageBatch, Unit> _stream;
        private IClientStreamWriter<MessageBatch> _streamWriter;

        public EndpointWriter(string host)
        {
            _host = host;
        }

        public async Task ReceiveAsync(IContext context)
        {
            var msg = context.Message;
            if (msg is Started)
            {
                await StartedAsync();
            }
            if (msg is Stopped)
            {
                await StoppedAsync();
            }
            if (msg is Restarting)
            {
                await RestartingAsync();
            }
            if (msg is IEnumerable<MessageEnvelope>)
            {
                var envelopes = msg as IEnumerable<MessageEnvelope>;
                await SendEnvelopesAsync(envelopes);
            }
        }

        private async Task SendEnvelopesAsync(IEnumerable<MessageEnvelope> envelopes)
        {
            var batch = new MessageBatch();
            batch.Envelopes.AddRange(envelopes);

            await _streamWriter.WriteAsync(batch);
        }

        private async Task RestartingAsync()
        {
            await _channel.ShutdownAsync();
        }

        private async Task StoppedAsync()
        {
            await _channel.ShutdownAsync();
        }

        private Task StartedAsync()
        {
            _channel = new Channel(_host, ChannelCredentials.Insecure);
            _client = new Remoting.RemotingClient(_channel);
            _stream = _client.Receive();
            _streamWriter = _stream.RequestStream;
            return Actor.Done;
        }
    }
}