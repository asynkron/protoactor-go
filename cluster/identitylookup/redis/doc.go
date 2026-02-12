// Package redis implements a Redis-backed identity storage for Proto.Actor
// cluster identity management.
//
// It implements the cluster.StorageLookup interface using Redis hashes and sets,
// with Lua scripts for atomic operations. The key schema follows the C#
// Proto.Cluster.Identity.Redis reference implementation:
//
//   - Identity hashes:  {clusterName}:ci:{kind}/{identity}
//   - Member sets:      {clusterName}:mb:{memberID}
//
// Usage:
//
//	import (
//	    goredis "github.com/redis/go-redis/v9"
//	    redisidentity "github.com/asynkron/protoactor-go/cluster/identitylookup/redis"
//	    "github.com/asynkron/protoactor-go/cluster/identitylookup/storage"
//	)
//
//	client := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
//	redisStorage := redisidentity.New("mycluster", client)
//	identityLookup := storage.New(redisStorage)
//
//	cfg := cluster.Configure("mycluster", provider, identityLookup, remoteCfg)
package redis
