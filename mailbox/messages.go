package mailbox

// ResumeMailbox is message sent by the actor system to resume mailbox processing.
//
// This will not be forwarded to the Receive method
type ResumeMailbox struct{}

// SuspendMailbox is message sent by the actor system to suspend mailbox processing.
//
// This will not be forwarded to the Receive method
type SuspendMailbox struct{}
