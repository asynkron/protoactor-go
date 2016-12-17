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
        Task NextAsync();
    }

    public class Context : IMessageInvoker, IContext
    {
        private IActor _actor;
        private HashSet<PID> _children;
        private int _receiveIndex;
        private ReceiveAsync[] _receivePlugins;
        private bool _restarting;
        private Stack<object> _stash;
        private bool _stopping;
        private SupervisionStrategy _supervisionStrategy;
        private HashSet<PID> _watchers;
        private HashSet<PID> _watching;

        private async Task ActorReceiveAsync(IContext ctx)
        {
            await _actor.ReceiveAsync(ctx);
        }

        public Context(Props props, PID parent)
        {

            Parent = parent;
            Props = props;
            _receivePlugins = new ReceiveAsync[] {}; 
            _watchers = null;
            _watching = null;
            Message = null;
            _actor = props.Producer();
        }

        public void InvokeSystemMessage(SystemMessage msg)
        {
        }

        public async Task InvokeUserMessageAsync(object msg)
        {
            _receiveIndex = 0;
            Message = msg;

            await NextAsync();
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

        public async Task NextAsync()
        {
            ReceiveAsync receive;
            if (_receiveIndex < _receivePlugins.Length)
            {
                receive = _receivePlugins[_receiveIndex];
                _receiveIndex++;
            }
            else
            {
                receive = ActorReceiveAsync;
            }

            await receive(this);
        }
    }
}