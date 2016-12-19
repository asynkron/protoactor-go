// -----------------------------------------------------------------------
//  <copyright file="EndpointWriterMailbox.cs" company="Asynkron HB">
//      Copyright (C) 2015-2016 Asynkron HB All rights reserved
//  </copyright>
// -----------------------------------------------------------------------

using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;

namespace GAM.Remoting
{
    internal static class MailboxStatus
    {
        public const int Idle = 0;
        public const int Busy = 1;
    }

    public class EndpointWriterMailbox : IMailbox
    {
        private readonly ConcurrentQueue<SystemMessage> _systemMessages = new ConcurrentQueue<SystemMessage>();
        private readonly ConcurrentQueue<object> _userMessages = new ConcurrentQueue<object>();
        private IDispatcher _dispatcher;
        private volatile bool _hasMoreMessages;
        private IMessageInvoker _invoker;

        private int _status = MailboxStatus.Idle;
        private bool _suspended;

        public void PostUserMessage(object msg)
        {
            _userMessages.Enqueue(msg);
            Schedule();
        }

        public void PostSystemMessage(SystemMessage sys)
        {
            _systemMessages.Enqueue(sys);
            Schedule();
        }

        public void RegisterHandlers(IMessageInvoker invoker, IDispatcher dispatcher)
        {
            _invoker = invoker;
            _dispatcher = dispatcher;
        }

        private async Task RunAsync()
        {
            _hasMoreMessages = false;
            var t = _dispatcher.Throughput;
            var batch = new List<MessageEnvelope>();

            SystemMessage sys;
            if (_systemMessages.TryDequeue(out sys))
            {
                if (sys is SuspendMailbox)
                {
                    _suspended = true;
                }
                if (sys is ResumeMailbox)
                {
                    _suspended = false;
                }
                _invoker.InvokeSystemMessage(sys);
            }
            if (!_suspended)
            {
                object msg;
                batch.Clear();
                while (_userMessages.TryDequeue(out msg))
                {
                    batch.Add((MessageEnvelope) msg);
                    if (batch.Count > 1000)
                    {
                        break;
                    }
                }


                if (batch.Count > 0)
                {
                    _hasMoreMessages = true;
                    await _invoker.InvokeUserMessageAsync(batch);
                }
            }


            Interlocked.Exchange(ref _status, MailboxStatus.Idle);

            if (_userMessages.Count > 0 || _systemMessages.Count > 0)
            {
                Schedule();
            }
        }

        protected void Schedule()
        {
            _hasMoreMessages = true;
            if (Interlocked.Exchange(ref _status, MailboxStatus.Busy) == MailboxStatus.Idle)
            {
                _dispatcher.Schedule(RunAsync);
            }
        }
    }
}