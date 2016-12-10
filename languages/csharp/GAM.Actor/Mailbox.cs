//-----------------------------------------------------------------------
// <copyright file="ConcurrentQueueMailbox.cs" company="Akka.NET Project">
//     Copyright (C) 2009-2016 Lightbend Inc. <http://www.lightbend.com>
//     Copyright (C) 2013-2016 Akka.NET project <https://github.com/akkadotnet/akka.net>
// </copyright>
//-----------------------------------------------------------------------

using System;
using System.Collections.Concurrent;
using System.Diagnostics;
using System.Threading;

namespace GAM
{
    internal static class MailboxStatus
    {
        /// <summary>
        ///     The idle
        /// </summary>
        public const int Idle = 0;

        /// <summary>
        ///     The busy
        /// </summary>
        public const int Busy = 1;
    }

    [Flags]
    public enum MailboxSuspendStatus
    {
        NotSuspended = 0,
        Supervision = 1,
        AwaitingTask = 2,
    }
    /// <summary>
    /// Class ConcurrentQueueMailbox.
    /// </summary>
    public class ConcurrentQueueMailbox
    {
        protected int status = MailboxStatus.Busy;  //HACK: Initially set the mailbox as busy in order for it not to scheduled until we want it to

        private volatile MailboxSuspendStatus _suspendStatus;
        public bool IsSuspended => _suspendStatus != MailboxSuspendStatus.NotSuspended;

        protected volatile bool hasUnscheduledMessages;

        internal bool HasUnscheduledMessages => hasUnscheduledMessages;

        public void Suspend()
        {
            Suspend(MailboxSuspendStatus.Supervision);
        }

        public void Resume()
        {
            _suspendStatus = MailboxSuspendStatus.NotSuspended;
            Schedule();
        }

        public void Suspend(MailboxSuspendStatus reason)
        {
            _suspendStatus |= reason;
        }

        public void Resume(MailboxSuspendStatus reason)
        {
            _suspendStatus &= ~reason;
            Schedule();
        }
        private readonly ConcurrentQueue<object> _systemMessages = new ConcurrentQueue<object>();
        private readonly ConcurrentQueue<object> _userMessages = new ConcurrentQueue<object>();

        private volatile bool _isClosed;

        private void Run()
        {
            if (_isClosed)
            {
                return;
            }


            //we are about to process all enqueued messages
            hasUnscheduledMessages = false;
            object envelope;

            //start with system messages, they have the highest priority
            while (_systemMessages.TryDequeue(out envelope))
            {
                dispatcher.SystemDispatch(ActorCell, envelope);
            }

            //we should process x messages in this run
            var left = dispatcher.Throughput;

            //try dequeue a user message
            while (!IsSuspended && !_isClosed && _userMessages.TryDequeue(out envelope))
            {
                //run the receive handler
                dispatcher.Dispatch(ActorCell, envelope);

                //check if any system message have arrived while processing user messages
                if (_systemMessages.TryDequeue(out envelope))
                {
                    //handle system message
                    dispatcher.SystemDispatch(ActorCell, envelope);
                    break;
                }
                left--;
                if (_isClosed)
                    return;

                //we are done processing messages for this run
                if (left == 0)
                {
                    break;
                }
            }

            Interlocked.Exchange(ref status, MailboxStatus.Idle);

            //there are still messages that needs to be processed
            if (_systemMessages.Count > 0 || (!IsSuspended && _userMessages.Count > 0))
            {
                //we still need has unscheduled messages for external info.
                //e.g. repointable actor ref uses it
                //TODO: will this be enough for external parties to work?
                hasUnscheduledMessages = true;

                //this is subject of a race condition
                //but that doesn't matter, since if the above "if" misses
                //the "Post" that adds the new message will still schedule
                //this specific call is just to deal with existing messages
                //that wasn't scheduled due to dispatcher throughput being reached
                //or system messages arriving during user message processing
                Schedule();
            }
        }


        /// <summary>
        /// Schedules this instance.
        /// </summary>
        protected void Schedule()
        {
            //only schedule if we idle
            if (Interlocked.Exchange(ref status, MailboxStatus.Busy) == MailboxStatus.Idle)
            {
                dispatcher.Schedule(Run);
            }
        }

        /// <summary>
        /// Posts the specified envelope.
        /// </summary>
        /// <param name="receiver"></param>
        /// <param name="envelope"> The envelope. </param>
        public override void Post(IActorRef receiver, object envelope)
        {
            if (_isClosed)
                return;

            hasUnscheduledMessages = true;
            if (envelope.Message is ISystemMessage)
            {
                _systemMessages.Enqueue(envelope);
            }
            else
            {
                _userMessages.Enqueue(envelope);
            }

            Schedule();
        }
    }
}

