using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace GAM
{
   
    public class HashedConcurrentDictionary
    {
        public class Partition : ConcurrentDictionary<string, ActorRef> { }

        private Partition[] _partitions = new Partition[1024];
        public HashedConcurrentDictionary()
        {
            for (int i = 0; i < _partitions.Length; i++)
            {
                _partitions[i] = new Partition();
            }
        }

        private Partition GetPartition(string key)
        {
            var hash = Math.Abs(key.GetHashCode()) % 1024;
            var p = _partitions[hash];
            return p;
        }

        public bool TryAdd(string key, ActorRef reff)
        {
            var p = GetPartition(key);
            return p.TryAdd(key, reff);
        }

        public bool TryGetValue(string key, out ActorRef aref)
        {
            var p = GetPartition(key);
            return p.TryGetValue(key, out aref);
        }
    }
}
