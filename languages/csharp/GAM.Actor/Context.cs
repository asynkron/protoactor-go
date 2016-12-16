// //-----------------------------------------------------------------------
// // <copyright file="Context.cs" company="Asynkron HB">
// //     Copyright (C) 2015-2016 Asynkron HB All rights reserved
// // </copyright>
// //-----------------------------------------------------------------------

using System.Collections.Generic;
using System.Threading.Tasks;

namespace GAM
{
    public interface IMessageInvoker
    {
        void InvokeSystemMessage(SystemMessage msg);
        Task InvokeUserMessageAsync(object msg);
    }

    public interface IContext
    {
    }

    public class Context : IMessageInvoker, IContext
    {
        private IActor _actor;
        private HashSet<PID> _children;
        private object _message;
        private PID _parent;
        private Props _props;
        private int _receiveIndex;
        private Receive[] _receivePlugins;
        private bool _restarting;
        private PID _self;
        private Stack<object> _stash;
        private bool _stopping;
        private SupervisionStrategy _supervisionStrategy;
        private HashSet<PID> _watchers;
        private HashSet<PID> _watching;

        public void InvokeSystemMessage(SystemMessage msg)
        {
        }

        public async Task InvokeUserMessageAsync(object msg)
        {
            await Task.Yield();
        }
    }
}