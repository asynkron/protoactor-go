// Package protodynamo provides a DynamoDB-backed persistence provider for
// Proto.Actor (protoactor-go).
//
// It implements the persistence.ProviderState interface, storing events and
// snapshots in two separate DynamoDB tables. Protobuf messages are serialized
// using [google.golang.org/protobuf/proto] and stored as binary attributes,
// alongside a DataType string attribute that records the protobuf full name
// for deserialization via the global protobuf type registry.
//
// # Table Schema
//
// Events table (default name "protoactor_events"):
//
//	ActorName  (S) – partition key
//	EventIndex (N) – sort key
//	Data       (B) – proto.Marshal output
//	DataType   (S) – proto.MessageName
//
// Snapshots table (default name "protoactor_snapshots"):
//
//	ActorName      (S) – partition key
//	SnapshotIndex  (N) – sort key
//	Data           (B) – proto.Marshal output
//	DataType       (S) – proto.MessageName
//
// # Usage
//
//	cfg, _ := config.LoadDefaultConfig(ctx)
//	client := dynamodb.NewFromConfig(cfg)
//	provider, _ := protodynamo.New(
//	    protodynamo.WithClient(client),
//	    protodynamo.WithSnapshotInterval(5),
//	)
//	// optionally create tables
//	provider.CreateTablesIfNotExist(ctx)
package protodynamo
