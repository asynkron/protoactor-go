package actor
import "sync/atomic"

const MailboxIdle int32 = 0
const MailboxBussy int32 = 1
const MailboxHasMoreMessages int32 = 1
const MailboxHasNoMessages int32 = 0

type Mailbox struct {
	userMailbox   chan interface{}
	systemMailbox chan interface{}
    schedulerStatus int32
	hasMoreMessages int32
    actorCell   *ActorCell
}

func (mailbox *Mailbox) schedule() {
	swapped := atomic.CompareAndSwapInt32(&mailbox.schedulerStatus, MailboxIdle, MailboxBussy)
	atomic.StoreInt32(&mailbox.hasMoreMessages, MailboxHasMoreMessages) //we have more messages to process
	if swapped {
		go mailbox.processMessages()
	}
}

func (mailbox *Mailbox) processMessages() {
	atomic.StoreInt32(&mailbox.hasMoreMessages, MailboxHasNoMessages)
	for i := 0; i < 30; i++ {
		select {
		case sysMsg := <-mailbox.systemMailbox:
			//prioritize system messages
			mailbox.actorCell.invokeSystemMessage(sysMsg)
		default:
			//if no system message is present, try read user message
			select {
			case userMsg := <-mailbox.userMailbox:
				mailbox.actorCell.invokeUserMessage(userMsg)
			default:
			}
		}
	}
	atomic.StoreInt32(&mailbox.schedulerStatus, MailboxIdle)
	hasMore := atomic.LoadInt32(&mailbox.hasMoreMessages) //was there any messages scheduled since we began processing?
	status := atomic.LoadInt32(&mailbox.schedulerStatus)  //have there been any new scheduling of the mailbox? (e.g. race condition from the two above lines)
	if hasMore == MailboxHasMoreMessages && status == MailboxIdle {
		swapped := atomic.CompareAndSwapInt32(&mailbox.schedulerStatus, MailboxIdle, MailboxBussy)
		if swapped {
			go mailbox.processMessages()
		}
	}
}