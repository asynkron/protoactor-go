package actor

// Directive is an enum for supervision actions
type Directive int

// Directive determines how a supervisor should handle a faulting actor
const (
	// ResumeDirective instructs the supervisor to resume the actor and continue processing messages
	ResumeDirective Directive = iota

	// RestartDirective instructs the supervisor to discard the actor, replacing it with a new instance,
	// before processing additional messages
	RestartDirective

	// StopDirective instructs the supervisor to stop the actor
	StopDirective

	// EscalateDirective instructs the supervisor to escalate handling of the failure to the actor's parent supervisor
	EscalateDirective
)
