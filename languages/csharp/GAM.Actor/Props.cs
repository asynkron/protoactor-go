// //-----------------------------------------------------------------------
// // <copyright file="Props.cs" company="Asynkron HB">
// //     Copyright (C) 2015-2016 Asynkron HB All rights reserved
// // </copyright>
// //-----------------------------------------------------------------------

using System;

namespace GAM
{

    public sealed class Props
    {
        private Func<IActor> _actorProducer;
        private Func<IMailbox> _mailboxProducer;

        public Props WithDispatcher(IDispatcher dispatcher)
        {
            return Copy(dispatcher: dispatcher);
        }


        public Props Copy(Func<IActor> producer = null, IDispatcher dispatcher = null, Func<IMailbox> mailboxProducer = null )
        {
            return new Props()
            {
                _actorProducer = producer ?? _actorProducer,
                Dispatcher = dispatcher ?? Dispatcher,
                _mailboxProducer = mailboxProducer ?? _mailboxProducer,
            };
        }

        public Func<IMailbox> MailboxProducer => _mailboxProducer;

        public IDispatcher Dispatcher { get; private set; }
    }
}