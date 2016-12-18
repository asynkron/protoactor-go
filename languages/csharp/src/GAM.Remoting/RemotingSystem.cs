using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using Grpc.Core;

namespace GAM.Remoting
{
    public static class RemotingSystem
    {
        private static Server server;
        public static void Start(string host,int port)
        {
            server = new Server
            {
                Services = { Remoting.BindService(new EndpointReader()) },
                Ports = { new ServerPort(host, port, ServerCredentials.Insecure) }
            };
            server.Start();

            Console.WriteLine("[REMOTING] Starting GAM server on {0}:{1}" ,host,port);
        }
    }
}
