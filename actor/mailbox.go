package actor

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/AsynkronIT/protoactor-go/internal/queue/mpsc"
)

type MailboxStatistics interface {
	MailboxStarted()
	MessagePosted(message interface{})
	MessageReceived(message interface{})
	MailboxEmpty()
}

type MessageInvoker interface {
	InvokeSystemMessage(SystemMessage)
	InvokeUserMessage(interface{})
	EscalateFailure(who *PID, reason interface{}, message interface{})
}

type MailboxRunner func()

type MailboxProducer func() Mailbox

type Mailbox interface {
	PostUserMessage(message interface{})
	PostSystemMessage(message SystemMessage)
	RegisterHandlers(invoker MessageInvoker, dispatcher Dispatcher)
}

const (
	mailboxIdle int32 = iota
	mailboxRunning
)

const (
	mailboxHasNoMessages int32 = iota
	mailboxHasMoreMessages
)

type DefaultMailbox struct {
	userMailbox     MailboxQueue
	systemMailbox   *mpsc.Queue
	schedulerStatus int32
	hasMoreMessages int32
	invoker         MessageInvoker
	dispatcher      Dispatcher
	suspended       bool
	mailboxStats    []MailboxStatistics
}

func (m *DefaultMailbox) PostUserMessage(message interface{}) {
	for _, ms := range m.mailboxStats {
		ms.MessagePosted(message)
	}
	m.userMailbox.Push(message)
	m.schedule()
}

func (m *DefaultMailbox) PostSystemMessage(message SystemMessage) {
	m.systemMailbox.Push(message)
	m.schedule()
}

func (m *DefaultMailbox) schedule() {
	atomic.StoreInt32(&m.hasMoreMessages, mailboxHasMoreMessages) //we have more messages to process
	if atomic.CompareAndSwapInt32(&m.schedulerStatus, mailboxIdle, mailboxRunning) {
		m.dispatcher.Schedule(m.processMessages)
	}
}

func (m *DefaultMailbox) processMessages() {
	//we are about to start processing messages, we can safely reset the message flag of the mailbox
	atomic.StoreInt32(&m.hasMoreMessages, mailboxHasNoMessages)

process:
	m.run()

	// set mailbox to idle
	atomic.StoreInt32(&m.schedulerStatus, mailboxIdle)

	// check if there are still messages to process (sent after the message loop ended)
	if atomic.SwapInt32(&m.hasMoreMessages, mailboxHasNoMessages) == mailboxHasMoreMessages {
		// try setting the mailbox back to running
		if atomic.CompareAndSwapInt32(&m.schedulerStatus, mailboxIdle, mailboxRunning) {
			goto process
		}
	}

	for _, ms := range m.mailboxStats {
		ms.MailboxEmpty()
	}
}

func identifyPanic() string {
	var name, file string
	var line int
	var pc [16]uintptr

	n := runtime.Callers(3, pc[:])
	for _, pc := range pc[:n] {
		log.Printf("%d", pc)
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}
		file, line = fn.FileLine(pc)
		name = fn.Name()
		if !strings.HasPrefix(name, "runtime.") {
			break
		}
	}

	switch {
	case name != "":
		return fmt.Sprintf("%v:%v", name, line)
	case file != "":
		return fmt.Sprintf("%v:%v", file, line)
	}

	return fmt.Sprintf("pc:%x", pc)
}

func (m *DefaultMailbox) run() {
	var msg interface{}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ACTOR] '%v' Recovering from: %v. Detailed stack: %v", m.invoker, r, identifyPanic())

			m.invoker.EscalateFailure(nil, r, msg)
		}
	}()

	i, t := 0, m.dispatcher.Throughput()
	for {
		if i > t {
			i = 0
			runtime.Gosched()
		}

		i++

		// keep processing system messages until queue is empty
		if msg = m.systemMailbox.Pop(); msg != nil {
			sys, _ := msg.(SystemMessage)
			switch sys.(type) {
			case *SuspendMailbox:
				m.suspended = true
			case *ResumeMailbox:
				m.suspended = false
			}

			m.invoker.InvokeSystemMessage(sys)
			continue
		}

		// didn't process a system message, so break until we are resumed
		if m.suspended {
			return
		}

		if msg = m.userMailbox.Pop(); msg != nil {
			m.invoker.InvokeUserMessage(msg)
			for _, ms := range m.mailboxStats {
				ms.MessageReceived(msg)
			}
		} else {
			return
		}
	}

}

func (m *DefaultMailbox) RegisterHandlers(invoker MessageInvoker, dispatcher Dispatcher) {
	m.invoker = invoker
	m.dispatcher = dispatcher
	for _, ms := range m.mailboxStats {
		ms.MailboxStarted()
	}
}
