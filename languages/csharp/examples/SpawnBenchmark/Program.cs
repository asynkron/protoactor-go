// -----------------------------------------------------------------------
//  <copyright file="Program.cs" company="Asynkron HB">
//      Copyright (C) 2015-2016 Asynkron HB All rights reserved
//  </copyright>
// -----------------------------------------------------------------------

using System;
using System.Diagnostics;
using System.Threading.Tasks;
using GAM;
using System.Threading;

namespace SpawnBenchmark
{
    internal class Request
    {
        public long Div;
        public long Num;
        public long Size;
    }

    internal class MyActor : IActor
    {
        public static Props props = Actor.FromProducer(() => new MyActor());
        private long Replies;
        private PID ReplyTo;
        private long Sum;

        public Task ReceiveAsync(IContext context)
        {
            switch (context.Message)
            {
                case Request r:
                    if (r.Size == 1)
                    {
                        context.Respond(r.Num);
                        return Actor.Done;
                    }
                    Replies = r.Div;
                    ReplyTo = context.Sender;
                    for (var i = 0; i < r.Div; i++)
                    {
                        var child = context.Spawn(props);
                        child.Request(new Request
                        {
                            Num = r.Num + i * (r.Size / r.Div),
                            Size = r.Size / r.Div,
                            Div = r.Div
                        }, context.Self);
                    }

                    return Actor.Done;
                case Int64 i:
                    Sum += i;
                    Replies--;
                    if (Replies == 0)
                    {
                        ReplyTo.Tell(Sum);
                    }
                    return Actor.Done;
                default:
                    return Actor.Done;
            }
        }
    }

    internal class Program
    {
        private static void Main()
        {
            var pid = Actor.Spawn(MyActor.props);
            var sw = Stopwatch.StartNew();
            var t = pid.RequestAsync<long>(new Request
            {
                Num = 0,
                Size = 1000000,
                Div = 10
            });
            t.ConfigureAwait(false);
            var res = t.Result;
            Console.WriteLine(sw.Elapsed);

            Console.WriteLine(res);
            Console.ReadLine();
        }
    }
}