/*
Package actor declares the types used to represent actors in the Actor Model.

The actors model provide a high level abstraction for writing concurrent and distributed systems. This approach
simplifies the burden imposed on engineers, such as explicit locks and concurrent access to shared state, as actors
receive messages synchronously.

The following quote from Wikipedia distills the definition of an actor down to its essence

	In response to a message that it receives, an actor can: make local decisions, create more actors,
	send more messages, and determine how to respond to the next message received.

Creating Actors

Props provide the building blocks for declaring how actors should be created. The following example defines an actor
using a function literal to process messages:

	var props Props = actor.FromFunc(func(c Context) {
		// process messages
	})

Spawn and SpawnNamed use the given props to create new instances of an actor. Once spawned, the actor is
ready to process incoming messages. To spawn an actor with a unique name, use

	pid := actor.Spawn(props)

The result of calling Spawn is a unique PID or process identifier.

Communicating With Actors

A PID is the primary interface for sending messages to actors. The PID.Tell method is used to send an asynchronous
message to the actor associated to the PID:

	pid.Tell("Hello World")

Depending on the requirements, communication between actors can take place synchronously or asynchronously. Regardless
of the circumstances, actors always communicate via a PID.

For synchronous communication, an actor will use a Future and wait for the result before continuing.

*/
package actor
