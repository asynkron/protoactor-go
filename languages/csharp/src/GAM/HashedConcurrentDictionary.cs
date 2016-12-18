// -----------------------------------------------------------------------
//  <copyright file="HashedConcurrentDictionary.cs" company="Asynkron HB">
//      Copyright (C) 2015-2016 Asynkron HB All rights reserved
//  </copyright>
// -----------------------------------------------------------------------

using System;
using System.Collections.Concurrent;

namespace GAM
{
    public class HashedConcurrentDictionary
    {
        private const int HashSize = 1024;
        private readonly Partition[] _partitions = new Partition[HashSize];

        static UInt64 CalculateHash(string read)
        {
            UInt64 hashedValue = 3074457345618258791ul;
            for (int i = 0; i < read.Length; i++)
            {
                hashedValue += read[i];
                hashedValue *= 3074457345618258799ul;
            }
            return hashedValue;
        }

        public HashedConcurrentDictionary()
        {
            for (var i = 0; i < _partitions.Length; i++)
            {
                _partitions[i] = new Partition();
            }
        }

        private Partition GetPartition(string key)
        {
            var hash = Math.Abs(key.GetHashCode())%HashSize;
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

        public bool TryRemove(string key, out ActorRef aref)
        {
            var p = GetPartition(key);
            return p.TryRemove(key, out aref);
        }

        public class Partition : ConcurrentDictionary<string, ActorRef>
        {
        }
    }
}