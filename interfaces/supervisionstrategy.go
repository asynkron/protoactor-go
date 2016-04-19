package interfaces

type Directive int
const (
    Resume Directive = iota
    Restart
    Stop
    Escalate
)

type Decider func(child ActorRef, cause interface{}) Directive

type SupervisionStrategy interface {
    Handle(child ActorRef, cause interface{}) Directive
}