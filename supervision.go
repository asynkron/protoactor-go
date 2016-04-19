package actor

type OneForOneStrategy struct {
	maxNrOfRetries              int
	withinTimeRangeMilliseconds int
	decider                     Decider
}

func (strategy *OneForOneStrategy) Handle(child ActorRef, reason interface{}) Directive {
	return strategy.decider(child, reason)
}

func NewOneForOneStrategy(maxNrOfRetries int, withinTimeRangeMilliseconds int, decider Decider) SupervisionStrategy {
	return &OneForOneStrategy{
		maxNrOfRetries:              maxNrOfRetries,
		withinTimeRangeMilliseconds: withinTimeRangeMilliseconds,
		decider:                     decider,
	}
}

func DefaultDecider(child ActorRef, reason interface{}) Directive {
	return RestartDirective
}

var defaultStrategy SupervisionStrategy = NewOneForOneStrategy(10, 30000, DefaultDecider)

func DefaultStrategy() SupervisionStrategy {
	return defaultStrategy
}
