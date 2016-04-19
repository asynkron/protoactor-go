package actor

type Directive int
const (
    ResumeDirective Directive = iota
    RestartDirective
    StopDirective
    EscalateDirective
)

type Decider func(child ActorRef, cause interface{}) Directive

type SupervisionStrategy interface {
    Handle(child ActorRef, cause interface{}) Directive
}