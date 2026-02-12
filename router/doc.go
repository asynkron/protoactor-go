// Package router provides message routing patterns for distributing work across multiple actors.
//
// The router package implements common routing strategies including round-robin, broadcast,
// random selection, and consistent hashing. Routers can operate in group mode (routing to
// existing actors) or pool mode (managing a pool of worker actors).
//
// Key types:
//   - State: Interface for router implementations that route messages to routees
//   - RouterConfig: Configuration interface for creating router states
//   - GroupRouter: Routes messages to an existing set of actors
//   - PoolRouter: Creates and manages a pool of worker actors
//   - Config: Functions for creating specific router configurations (NewRoundRobinPool, NewBroadcastGroup, etc.)
//
// Routing strategies:
//   - Round-robin: Distributes messages evenly across routees in rotation
//   - Broadcast: Sends each message to all routees
//   - Random: Selects a random routee for each message
//   - Consistent hash: Routes based on message hash for stable routing
//
// Basic usage:
//
//	// Create a round-robin pool of 5 workers
//	props := router.NewRoundRobinPool(5).
//	    WithProducer(func() actor.Actor { return &WorkerActor{} })
//	pid := system.Root.Spawn(props)
//
//	// Create a broadcast group routing to existing actors
//	routees := actor.NewPIDSet(pid1, pid2, pid3)
//	props := router.NewBroadcastGroup(routees)
//	router := system.Root.Spawn(props)
package router
