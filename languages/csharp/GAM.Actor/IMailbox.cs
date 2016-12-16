// //-----------------------------------------------------------------------
// // <copyright file="IMailbox.cs" company="Asynkron HB">
// //     Copyright (C) 2015-2016 Asynkron HB All rights reserved
// // </copyright>
// //-----------------------------------------------------------------------

using System.Collections.Concurrent;
using System.Threading;
using System.Threading.Tasks;

namespace GAM
{
    internal static class MailboxStatus
    {
        public const int Idle = 0;
        public const int Busy = 1;
    }

    internal static class MailboxMessages
    {
        public const int MailboxHasNoMessages = 0;
        public const int MailboxHasMoreMessages = 1;
    }

    public interface IMailbox
    {
        void PostUserMessage(object msg);
        void PostSystemMessage(SystemMessage sys);
        void RegisterHandlers(IMessageInvoker  invoker, IDispatcher dispatcher);
    }

    public class DefaultMailbox : IMailbox
    {
        private readonly ConcurrentQueue<SystemMessage> _systemMessages = new ConcurrentQueue<SystemMessage>();
        private readonly ConcurrentQueue<object> _userMessages = new ConcurrentQueue<object>();
        private IDispatcher _dispatcher;
        private int _hasMoreMessages = MailboxMessages.MailboxHasNoMessages;
        private IMessageInvoker _invoker;

        private int _status = MailboxStatus.Idle;
        private bool _suspended;

        private async Task RunAsync()
        {
            //we are about to process all enqueued messages
            Interlocked.Exchange(ref _hasMoreMessages, MailboxMessages.MailboxHasNoMessages);
            var t = _dispatcher.Throughput;


            for (var i = 0; i < t; i++)
            {
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
                    continue;
                }
                if (_suspended)
                {
                    break;
                }
                object msg;
                if (_userMessages.TryDequeue(out msg))
                {
                    await _invoker.InvokeUserMessageAsync(msg);
                }
            }

            Interlocked.Exchange(ref _status, MailboxStatus.Idle);

            if (Interlocked.Exchange(ref _hasMoreMessages, MailboxMessages.MailboxHasNoMessages) ==
                MailboxMessages.MailboxHasMoreMessages)
            {
                Schedule();
            }
        }

        protected void Schedule()
        {
            Interlocked.Exchange(ref _hasMoreMessages, MailboxMessages.MailboxHasMoreMessages);
            if (Interlocked.Exchange(ref _status, MailboxStatus.Busy) == MailboxStatus.Idle)
            {
                _dispatcher.Schedule(RunAsync);
            }
        }

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

        public void RegisterHandlers(IMessageInvoker  invoker, IDispatcher dispatcher)
        {
            _invoker = invoker;
            _dispatcher = dispatcher;
        }
    }
}