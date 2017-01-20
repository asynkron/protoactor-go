package actor

type Directive int

// Directive determines how a supervisor should handle a failing actor
const (
	// ResumeDirective instructs the supervisor to resume the actor and continue processing messages for the actor
	ResumeDirective Directive = iota

	// RestartDirective instructs the supervisor to restart the actor before processing additional messages
	RestartDirective

	// StopDirective instructs the supervisor to stop the actor
	StopDirective

	// EscalateDirective instructs the supervisor to escalate handling of the failure to the actor's parent
	EscalateDirective
)
