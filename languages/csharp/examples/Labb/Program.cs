// -----------------------------------------------------------------------
//  <copyright file="Program.cs" company="Asynkron HB">
//      Copyright (C) 2015-2016 Asynkron HB All rights reserved
//  </copyright>
// -----------------------------------------------------------------------

using System;
using System.Threading.Tasks;
using GAM;

namespace Labb
{
    public class TheActor : IActor
    {
        public Task ReceiveAsync(IContext ctx)
        {
            Console.WriteLine(ctx.Message);
            return Task.FromResult(0);
        }
    }

    class Program
    {
        static void Main(string[] args)
        {
            var props = Actor.FromProducer(() => new TheActor());
            var a = Actor.Spawn(props);
            a.Tell("Hello");
            Console.ReadLine();
        }
    }
}