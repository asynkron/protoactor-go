package actor

type SupervisorEvent struct {
	Child     *PID
	Reason    interface{}
	Directive Directive
}
