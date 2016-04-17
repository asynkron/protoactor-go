package main

import "fmt"
import "bufio"
import "os"

func main() {
	myActor := ActorOf(new(MyActor))
	myActor.Tell(Hello{Name: "Roger"})
	myActor.Tell(Hello{Name: "Go"})
	bufio.NewReader(os.Stdin).ReadString('\n')
}

type ActorRef interface {
	Tell(message interface{})
	SendSystemMessage(message interface{})
}

type ChannelActorRef struct {
	actorCell *ActorCell
}

func (ref *ChannelActorRef) Tell(message interface{}) {
	ref.actorCell.userMailbox <- message
}

func (ref *ChannelActorRef) SendSystemMessage(message interface{}) {
	ref.actorCell.userMailbox <- message
}

type Actor interface {
	Receive(message interface{})
}

func ActorOf(actor Actor) ActorRef {
	userMailbox := make(chan interface{}, 100)
	systemMailbox := make(chan interface{}, 100)
	cell := &ActorCell{
		userMailbox:   userMailbox,
		systemMailbox: systemMailbox,
		actor:         actor,
	}
	ref := ChannelActorRef{
		actorCell: cell,
	}
	go func() {
		for {
			select {
			case sysMsg := <-systemMailbox:
                //prioritize system messages
				cell.invokeSystemMessage(sysMsg)
			default:
				//if no system message is present, try read user message
				select {
				case userMsg := <-userMailbox:
					cell.invokeUserMessage(userMsg)
				default:
				}
			}
		}
	}()

	return &ref
}

type ActorCell struct {
	userMailbox   chan interface{}
	systemMailbox chan interface{}
	actor         Actor
}

func (cell *ActorCell) invokeSystemMessage(message interface{}) {
	fmt.Printf("Received system message %v\n", message)
}

func (cell *ActorCell) invokeUserMessage(message interface{}) {
	cell.actor.Receive(message)
}

type MyActor struct{ messageCount int }
type Hello struct{ Name string }

func (state *MyActor) Receive(message interface{}) {
	switch msg := message.(type) {
	default:
		fmt.Printf("unexpected type %T\n", msg) // %T prints whatever type t has
	case Hello:
		fmt.Printf("Hello %v\n", msg.Name) // t has type bool
		state.messageCount++
	}
}
