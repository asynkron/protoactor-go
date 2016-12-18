// -----------------------------------------------------------------------
//  <copyright file="RemotingSystem.cs" company="Asynkron HB">
//      Copyright (C) 2015-2016 Asynkron HB All rights reserved
//  </copyright>
// -----------------------------------------------------------------------

using System;
using System.Collections.Generic;
using System.Linq;
using Google.Protobuf.Reflection;
using Grpc.Core;

namespace GAM.Remoting
{
    public static class RemotingSystem
    {
        private static Server server;

        public static void Start(string host, int port)
        {
            Serialization.Init();
            var addr = host + ":" + port;
            ProcessRegistry.Instance.Host = addr;

            server = new Server
            {
                Services = {Remoting.BindService(new EndpointReader())},
                Ports = {new ServerPort(host, port, ServerCredentials.Insecure)}
            };
            server.Start();

            Console.WriteLine("[REMOTING] Starting GAM server on {0}", addr);
        }
    }
}