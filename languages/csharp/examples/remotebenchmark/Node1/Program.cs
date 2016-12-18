using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using GAM.Remoting;
using Messages;

namespace Node1
{
    class Program
    {
        static void Main(string[] args)
        {
            var m = new StartRemote();
            RemotingSystem.Start("0.0.0.0",8080);
            Console.ReadLine();
        }
    }
}
