// -----------------------------------------------------------------------
//  <copyright file="Context.cs" company="Asynkron HB">
//      Copyright (C) 2015-2016 Asynkron HB All rights reserved
//  </copyright>
// -----------------------------------------------------------------------

using System;
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
        object Message { get; }
        PID Sender { get; }

        void Respond(object msg);
        PID[] Children();

        void Stash();
        Task NextAsync();
        PID Spawn(Props props);
    }

    public class Context : IMessageInvoker, IContext
    {
        private IActor _actor;
        private HashSet<PID> _children;
        private object _message;
        private int _receiveIndex;
        private ReceiveAsync[] _receivePlugins;
        private bool _restarting;
        private Stack<object> _stash;
        private bool _stopping;
        private SupervisionStrategy _supervisionStrategy;
        private HashSet<PID> _watchers;
        private HashSet<PID> _watching;
        private bool _isRestarting;
        private bool _isStopping;

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

        public PID[] Children()
        {
            return _children.ToArray();
        }

        public PID Parent { get; }
        public PID Self { get; internal set; }
        public Props Props { get; }

        public object Message
        {
            get
            {
                var r = _message as Request;
                return r != null ? r.Message : _message;
            }
            private set { _message = value; }
        }

        public PID Sender => (_message as Request)?.Sender;

        public void Stash()
        {
            if (_stash == null)
            {
                _stash = new Stack<object>();
            }
            _stash.Push(Message);
        }

        public Task NextAsync()
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

            return receive(this);
        }

        public void Respond(object msg)
        {
            Sender.Tell(msg);
        }

        public PID Spawn(Props props)
        {
            var id = ProcessRegistry.Instance.GetAutoId();

            return SpawnNamed(props, id);
        }

        public void InvokeSystemMessage(SystemMessage msg)
        {
            if (msg is Stop)
            {
                HandleStop();
            }
        }

        private void HandleStop()
        {
            _isRestarting = false;
            _isStopping = true;
            //this is intentional
            InvokeUserMessageAsync(Stopping.Instance).Wait();
            if (_children != null)
            {
                foreach (var child in _children)
                {
                    child.Stop();
                }
            }
            TryRestartOrTerminate();
        }

        private void TryRestartOrTerminate()
        {
            if (_children != null)
            {
                if (_children.Count > 0)
                {
                    return;
                }
            }

            if (_restarting)
            {
                Restart();
                return;
            }

            if (_stopping)
            {
                Stopped();
            }
        }

        private void Stopped()
        {
           ProcessRegistry.Instance.Remove(Self);
            //This is intentional
            InvokeUserMessageAsync(GAM.Stopped.Instance).Wait();
            //Notify watchers
        }

        private void Restart()
        {
            throw new NotImplementedException();
        }

        public async Task InvokeUserMessageAsync(object msg)
        {
            try
            {
                _receiveIndex = 0;
                Message = msg;

                await NextAsync();
            }
            catch (Exception x)
            {
                if (Parent == null)
                {
                }
                else
                {
                    Self.SendSystemMessage(new SuspendMailbox());
                }
                //handle supervision
            }
        }

        private Task ActorReceiveAsync(IContext ctx)
        {
            return _actor.ReceiveAsync(ctx);
        }

        public PID SpawnNamed(Props props, string name)
        {
            string fullname;
            if (Parent != null)
            {
                fullname = Parent.Id + "/" + name;
            }
            else
            {
                fullname = name;
            }

            var pid = Actor.InternalSpawn(props, fullname, Self);
            if (_children == null)
            {
                _children = new HashSet<PID>();
            }
            _children.Add(pid);
            Watch(pid);
            return pid;
        }

        private void Watch(PID who)
        {
            who.SendSystemMessage(new Watch(Self));
        }
    }
}