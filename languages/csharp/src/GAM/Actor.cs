// //-----------------------------------------------------------------------
// // <copyright file="Actor.cs" company="Asynkron HB">
// //     Copyright (C) 2015-2016 Asynkron HB All rights reserved
// // </copyright>
// //-----------------------------------------------------------------------

using System;
using System.Threading.Tasks;

namespace GAM
{
    public static class Actor
    {
        public static Props FromProducer(Func<IActor> producer)
        {
            return new Props().Copy(producer = producer);
        }

        public static PID Spawn(Props props)
        {
            var name = ProcessRegistry.Instance.GetAutoId();
            return spawn(props, name, null);
        }

        public static PID SpawnNamed(Props props, string name)
        {
            return spawn(props, name, null);
        }

        internal static PID spawn(Props props, string name, PID parent)
        {
            var ctx = new Context(props, parent);
            var mailbox = props.MailboxProducer();
            var dispatcher = props.Dispatcher;
            var reff = new LocalActorRef(mailbox);
            var res = ProcessRegistry.Instance.TryAdd(name,reff);
            if (res.Item2)
            {
                mailbox.RegisterHandlers(ctx, dispatcher);
                ctx.Self = res.Item1;
                ctx.InvokeUserMessageAsync(new Started ());
            }
            return res.Item1;
        }
    }

    public interface IActor
    {
        Task ReceiveAsync(IContext context);
    }
}