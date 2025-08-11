package actor

// CapturedContext holds a message envelope along with the context it was captured from.
type CapturedContext struct {
	MessageEnvelope *MessageEnvelope
	Context         Context
}

// Receive reprocesses the captured message on the captured context.
// It captures the current context state before processing and restores it afterwards.
func (cc *CapturedContext) Receive() {
	current := cc.Context.Capture()
	cc.Context.Receive(cc.MessageEnvelope)
	current.Apply()
}

// Apply restores the stored message to the actor context so it can be re-processed by the actor.
func (cc *CapturedContext) Apply() {
	cc.Context.Apply(cc)
}
