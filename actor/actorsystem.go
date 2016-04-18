
package actor

// import (
// 	"bufio"
// 	"os"
// 	"sync"
// )

// type ActorSystem struct {
// 	RootGuardian ActorRef
// 	Terminated   sync.WaitGroup
// }

// type RootGuardianActor struct {
// 	Terminated sync.WaitGroup
// }

// func (state *RootGuardianActor) Receive(context *Context) {
// 	switch msg := context.Message.(type) {
// 	case CreateActor:
// 		ref := context.ActorOf(msg.Props)
// 		msg.ReplyTo.Tell(ref)
// 	case Stopped:
// 		state.Terminated.Done()
// 	}
// }

// func NewActorSystem() *ActorSystem {

// 	system := ActorSystem{}
// 	system.Terminated.Add(1)

// 	newRootGuardian := func() Actor {
// 		return &RootGuardianActor{
// 			Terminated: system.Terminated,
// 		}
// 	}
// 	system.RootGuardian = spawn(Props(newRootGuardian))
// 	return &system
// }

// func (actorSystem *ActorSystem) ActorOf(props PropsValue) ActorRef {
// 	future := NewFutureActorRef()
// 	createActor := CreateActor{
// 		Props:   props,
// 		ReplyTo: future,
// 	}
// 	actorSystem.RootGuardian.Tell(createActor)
// 	res := <-future.Result()
// 	return res.(ActorRef)
// }

// func (actorSystem *ActorSystem) AwaitTermination() {
// 	// defer func() {
// 	// 	if r := recover(); r != nil {
// 	// 		fmt.Println("Recovered in f", r)
// 	// 	}
// 	// }()
// 	//actorSystem.Terminated.Wait()
// 	reader := bufio.NewReader(os.Stdin)
// 	reader.ReadString('\n')
// }
// 