// -----------------------------------------------------------------------
//  <copyright file="EventStream.cs" company="Asynkron HB">
//      Copyright (C) 2015-2016 Asynkron HB All rights reserved
//  </copyright>
// -----------------------------------------------------------------------

using System;
using System.Collections.Concurrent;

namespace GAM
{
    public class EventStream
    {
        public static readonly EventStream Instance = new EventStream();

        private readonly ConcurrentDictionary<Guid, Action<object>> _subscriptions =
            new ConcurrentDictionary<Guid, Action<object>>();

        public EventStream()
        {
            Subscribe(msg =>
            {
                var letter = msg as DeadLetter;
                if (letter != null)
                {
                    Console.WriteLine("[DeadLetter] {0} got {1}", letter.Pid, letter.Message);
                }
            });
        }

        public void Subscribe(Action<object> action)
        {
            var sub = Guid.NewGuid();
            _subscriptions.TryAdd(sub, action);
        }

        public void Publish(object msg)
        {
            foreach (var sub in _subscriptions)
            {
                sub.Value(msg);
            }
        }
    }
}