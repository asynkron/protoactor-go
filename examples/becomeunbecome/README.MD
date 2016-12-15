# Switchable Behaviors

Actors have the power to switch their behaviors at any point in time. This is usually referred as *becoming something*, 
as in *the actor becomes busy* or *the actor becomes connected*.

This is accomplished by replacing the method that handles messages inside the actor using `Become` or `BecomeStacked`. 
These methods accept a delegate that will handle the next messages until you decide to replace it again.

This is a powerful concept that is behind other features like Finite State Machines.

> **Note:**<br /> When you change the actor behavior, the new behaviour will take effect for all subsequent messages 
until the behaviour is changed again. The current message will continue processing with the existing behaviour. 
You can use Stashing to reprocess the current message with the new behavior.

## API

The API to change behaviors is available to the actor instance is very simple:

* `Become` - Replaces the message handler with the specified delegate;
* `BecomeStacked` - Adds the specified message handler to the top of the behavior stack, while maintaining the previous ones;
* `UnbecomeStacked` - Reverts to the previous message handler from the stack (only works with BecomeStacked);

```go
...

type Become struct{}
type Hello struct{ Who string }
type BecomeActor struct{}

//this is our first behavior
func (state *BecomeActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
		context.Become(state.Other)
	}
}

//this is our second behavior
func (state *BecomeActor) Other(context actor.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		fmt.Printf("%v, ey we are now handling messages in another behavior", msg.Who)
	}
}

func NewBecomeActor() actor.Actor {
	return &BecomeActor{}
}

func main() {
	props := actor.FromProducer(NewBecomeActor)
	pid := actor.Spawn(props)
	pid.Tell(Hello{Who: "Roger"})
	pid.Tell(Hello{Who: "Roger"})
	console.ReadLine()
}
```