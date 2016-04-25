package actor

type Directive int

const (
	ResumeDirective Directive = iota
	RestartDirective
	StopDirective
	EscalateDirective
)

type Decider func(child *PID, cause interface{}) Directive

type SupervisionStrategy interface {
	Handle(child *PID, cause interface{}) Directive
}

type OneForOneStrategy struct {
	maxNrOfRetries              int
	withinTimeRangeMilliseconds int
	decider                     Decider
}

func (strategy *OneForOneStrategy) Handle(child *PID, reason interface{}) Directive {
	return strategy.decider(child, reason)
}

func NewOneForOneStrategy(maxNrOfRetries int, withinTimeRangeMilliseconds int, decider Decider) SupervisionStrategy {
	return &OneForOneStrategy{
		maxNrOfRetries:              maxNrOfRetries,
		withinTimeRangeMilliseconds: withinTimeRangeMilliseconds,
		decider:                     decider,
	}
}

func DefaultDecider(child *PID, reason interface{}) Directive {
	return RestartDirective
}

var defaultSupervisionStrategy SupervisionStrategy = NewOneForOneStrategy(10, 30000, DefaultDecider)

func DefaultSupervisionStrategy() SupervisionStrategy {
	return defaultSupervisionStrategy
}
