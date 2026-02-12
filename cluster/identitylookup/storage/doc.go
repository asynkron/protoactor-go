// Package storage provides IdentityStorageLookup, an adapter that bridges a
// cluster.StorageLookup backend (e.g., Redis, MongoDB) to the
// cluster.IdentityLookup interface expected by the Proto.Actor cluster.
//
// Usage:
//
//	import (
//	    "github.com/asynkron/protoactor-go/cluster/identitylookup/storage"
//	    redisidentity "github.com/asynkron/protoactor-go/cluster/identitylookup/redis"
//	)
//
//	redisStorage := redisidentity.New("mycluster", client)
//	identityLookup := storage.New(redisStorage)
//
//	cfg := cluster.Configure("mycluster", provider, identityLookup, remoteCfg)
package storage
