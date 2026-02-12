// Package protopg provides a PostgreSQL-backed persistence provider for
// protoactor-go. It implements the persistence.ProviderState interface,
// storing events and snapshots as protobuf-encoded byte arrays in two
// PostgreSQL tables.
//
// Events and snapshots are serialized using google.golang.org/protobuf/proto
// and deserialized using the global protobuf type registry, so all message
// types used with this provider must be registered (which happens
// automatically for generated protobuf types).
//
// # Usage
//
//	provider, err := protopg.New(
//	    protopg.WithConnectionString("postgres://user:pass@localhost:5432/mydb"),
//	    protopg.WithSnapshotInterval(10),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer provider.Close()
//
//	if err := provider.CreateSchemaIfNotExists(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Use provider with protoactor persistence mixin
//	props := actor.PropsFromProducer(myActorFactory,
//	    actor.WithReceiverMiddleware(persistence.Using(provider)),
//	)
package protopg
