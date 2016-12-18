// -----------------------------------------------------------------------
//  <copyright file="Actor.cs" company="Asynkron HB">
//      Copyright (C) 2015-2016 Asynkron HB All rights reserved
//  </copyright>
// -----------------------------------------------------------------------

using System;
using System.Threading.Tasks;

namespace GAM
{
    public static class Actor
    {
        public static readonly Task Done = Task.FromResult(0);

        public static Props FromProducer(Func<IActor> producer)
        {
            return new Props().Copy(producer = producer);
        }

        public static PID Spawn(Props props)
        {
            var name = ProcessRegistry.Instance.GetAutoId();
            return InternalSpawn(props, name, null);
        }

        public static PID SpawnNamed(Props props, string name)
        {
            return InternalSpawn(props, name, null);
        }

        internal static PID InternalSpawn(Props props, string name, PID parent)
        {
            var ctx = new Context(props, parent);
            var mailbox = props.MailboxProducer();
            var dispatcher = props.Dispatcher;
            var reff = new LocalActorRef(mailbox);
            var (pid,ok) = ProcessRegistry.Instance.TryAdd(name, reff);
            if (ok)
            {
                mailbox.RegisterHandlers(ctx, dispatcher);
                ctx.Self = pid;
                ctx.InvokeUserMessageAsync(new Started());
            }
            return pid;
        }
    }

    public interface IActor
    {
        Task ReceiveAsync(IContext context);
    }
}