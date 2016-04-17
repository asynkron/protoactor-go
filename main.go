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
	ref.actorCell.mailbox.userMailbox <- message
	ref.actorCell.schedule()
}

func (ref *ChannelActorRef) SendSystemMessage(message interface{}) {
	ref.actorCell.mailbox.userMailbox <- message
	ref.actorCell.schedule()
}

type Actor interface {
	Receive(message interface{})
}

func ActorOf(actor Actor) ActorRef {
	userMailbox := make(chan interface{}, 100)
	systemMailbox := make(chan interface{}, 100)
	mailbox := Mailbox{
		userMailbox:   userMailbox,
		systemMailbox: systemMailbox,
	}
	cell := &ActorCell{
		mailbox:         &mailbox,
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
		case sysMsg := <-cell.mailbox.systemMailbox:
			//prioritize system messages
			cell.invokeSystemMessage(sysMsg)
		default:
			//if no system message is present, try read user message
			select {
			case userMsg := <-cell.mailbox.userMailbox:
				cell.invokeUserMessage(userMsg)
			default:
			}
		}
	}
	atomic.StoreInt32(&cell.schedulerStatus, MailboxIdle)
	hasMore := atomic.LoadInt32(&cell.hasMoreMessages) //was there any messages scheduled since we began processing?
	status := atomic.LoadInt32(&cell.schedulerStatus)  //have there been any new scheduling of the mailbox? (e.g. race condition from the two above lines)
	if hasMore == MailboxHasMoreMessages && status == MailboxIdle {
		swapped := atomic.CompareAndSwapInt32(&cell.schedulerStatus, MailboxIdle, MailboxBussy)
		if swapped {
			go cell.processMessages()
		}
	}
}

type ActorCell struct {
	mailbox         *Mailbox
	actor           Actor
	schedulerStatus int32
	hasMoreMessages int32
}

type Mailbox struct {
	userMailbox   chan interface{}
	systemMailbox chan interface{}
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
