package main

import "fmt"
import "bufio"
import "os"
import "sync/atomic"

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
	ref.actorCell.schedule()
}

func (ref *ChannelActorRef) SendSystemMessage(message interface{}) {
	ref.actorCell.userMailbox <- message
	ref.actorCell.schedule()
}

type Actor interface {
	Receive(message interface{})
}

func ActorOf(actor Actor) ActorRef {
	userMailbox := make(chan interface{}, 100)
	systemMailbox := make(chan interface{}, 100)
	cell := &ActorCell{
		userMailbox:     userMailbox,
		systemMailbox:   systemMailbox,
		actor:           actor,
		hasMoreMessages: int32(0),
		schedulerStatus: int32(0),
	}
	ref := ChannelActorRef{
		actorCell: cell,
	}

	return &ref
}

const MailboxIdle int32 = 0
const MailboxBussy int32 = 1
const MailboxHasMoreMessages int32 = 1
const MailboxHasNoMessages int32 = 0

func (cell *ActorCell) schedule() {
	swapped := atomic.CompareAndSwapInt32(&cell.schedulerStatus, MailboxIdle, MailboxBussy)
	atomic.StoreInt32(&cell.hasMoreMessages, MailboxHasMoreMessages) //we have more messages to process
	if swapped {
		go cell.processMessages()
	}
}

func (cell *ActorCell) processMessages() {
	atomic.StoreInt32(&cell.hasMoreMessages, MailboxHasNoMessages)
	for i := 0; i < 30; i++ {
		select {
		case sysMsg := <-cell.systemMailbox:
			//prioritize system messages
			cell.invokeSystemMessage(sysMsg)
		default:
			//if no system message is present, try read user message
			select {
			case userMsg := <-cell.userMailbox:
				cell.invokeUserMessage(userMsg)
			default:
			}
		}
	}

	hasMore := atomic.LoadInt32(&cell.hasMoreMessages) //was there any messages scheduled since we began processing?
	atomic.StoreInt32(&cell.schedulerStatus, MailboxIdle)
	if hasMore == MailboxHasMoreMessages {
		atomic.StoreInt32(&cell.schedulerStatus, MailboxBussy)
		go cell.processMessages()
	}
}

type ActorCell struct {
	userMailbox     chan interface{}
	systemMailbox   chan interface{}
	actor           Actor
	schedulerStatus int32
	hasMoreMessages int32
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
