using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using GAM.Remoting;

namespace Node1
{
    class Program
    {
        static void Main(string[] args)
        {
            RemotingSystem.Start("0.0.0.0",8080);
            Console.ReadLine();
        }
    }
}
