// //-----------------------------------------------------------------------
// // <copyright file="Context.cs" company="Asynkron HB">
// //     Copyright (C) 2015-2016 Asynkron HB All rights reserved
// // </copyright>
// //-----------------------------------------------------------------------

using System.Collections.Generic;
using System.Linq;
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
        PID Parent { get; }
        PID Self { get; }
        Props Props { get; }
        PID[] Children();
        object Message { get; }
        void Stash();
        void Next();
    }

    public class Context : IMessageInvoker, IContext
    {
        private IActor _actor;
        private HashSet<PID> _children;
        private int _receiveIndex;
        private Receive[] _receivePlugins;
        private bool _restarting;
        private Stack<object> _stash;
        private bool _stopping;
        private SupervisionStrategy _supervisionStrategy;
        private HashSet<PID> _watchers;
        private HashSet<PID> _watching;


        public Context(Props props, PID parent)
        {
            Parent = parent;
            Props = props;
            _receivePlugins = null; //props.ReceivePlugins
            _watchers = null;
            _watching = null;
            Message = null;
        }

        public void InvokeSystemMessage(SystemMessage msg)
        {
        }

        public async Task InvokeUserMessageAsync(object msg)
        {
            await Task.Yield();
        }

        public PID[] Children()
        {
            return _children.ToArray();
        }

        public PID Parent { get; }
        public PID Self { get; internal set; }
        public Props Props { get; }
        public object Message { get; private set; }
        public void Stash()
        {
            if (_stash == null)
            {
                _stash = new Stack<object>();
            }
            _stash.Push(Message);
        }

        public void Next()
        {
            throw new System.NotImplementedException();
        }
    }
}