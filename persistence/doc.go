// Package persistence provides event sourcing and snapshotting for stateful actors.
//
// The persistence package allows actors to persist their state through events and snapshots,
// enabling recovery after restarts or crashes. It supports pluggable storage backends through
// the Provider interface and configurable snapshot strategies.
//
// Key types:
//   - Provider: Interface for persistence storage backends
//   - ProviderState: Combined interface for event and snapshot storage operations
//   - Mixin: A composable type that adds persistence capabilities to actors
//   - SnapshotStrategy: Interface for controlling when snapshots are taken
//   - EventStore: Interface for persisting and retrieving events
//   - SnapshotStore: Interface for persisting and retrieving snapshots
//   - InMemoryProvider: Built-in in-memory provider for testing
//
// Basic usage:
//
//	type MyActor struct {
//	    persistence.Mixin
//	    state int
//	}
//
//	func (a *MyActor) Receive(ctx actor.Context) {
//	    switch msg := ctx.Message().(type) {
//	    case *AddValue:
//	        a.PersistReceive(&ValueAdded{Amount: msg.Value})
//	        a.state += msg.Value
//	    case *ValueAdded:
//	        a.state += msg.Amount
//	    }
//	}
//
//	provider := persistence.NewInMemoryProvider()
//	props := actor.PropsFromProducer(func() actor.Actor { return &MyActor{} }).
//	    WithReceiverMiddleware(persistence.Using(provider))
package persistence
