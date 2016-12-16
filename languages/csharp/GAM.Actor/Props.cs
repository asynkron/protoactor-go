// //-----------------------------------------------------------------------
// // <copyright file="Props.cs" company="Asynkron HB">
// //     Copyright (C) 2015-2016 Asynkron HB All rights reserved
// // </copyright>
// //-----------------------------------------------------------------------

namespace GAM
{
    public delegate IActor ActorProducer();

    public sealed class Props
    {
        private ActorProducer _actorProducer;
        private IDispatcher _dispatcher;

        public Props WithDispatcher(IDispatcher dispatcher)
        {
            return Copy(dispatcher: dispatcher);
        }


        public Props Copy(ActorProducer producer = null, IDispatcher dispatcher = null)
        {
            return new Props()
            {
                _actorProducer = producer ?? _actorProducer,
                _dispatcher = dispatcher ?? _dispatcher,
            };
        }
    }
}