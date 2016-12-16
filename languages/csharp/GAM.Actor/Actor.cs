// //-----------------------------------------------------------------------
// // <copyright file="Actor.cs" company="Asynkron HB">
// //     Copyright (C) 2015-2016 Asynkron HB All rights reserved
// // </copyright>
// //-----------------------------------------------------------------------

using System.Threading.Tasks;

namespace GAM
{
    public static class Actor
    {
        public static Props FromProducer(ActorProducer producer)
        {
            return new Props().Copy(producer = producer);
        }

        public static PID Spawn(Props props)
        {
            return null;
        }

        public static PID SpawnNamed(Props props, string name)
        {
            return null;
        }
    }

    public interface IActor
    {
        Task ReceiveAsync(IContext context);
    }
}