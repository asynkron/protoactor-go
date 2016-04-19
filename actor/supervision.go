package actor

import (
	"github.com/rogeralsing/goactor/interfaces"
)

type OneForOneStrategy struct {
	maxNrOfRetries              int
	withinTimeRangeMilliseconds int
	decider                     interfaces.Decider
}

func (strategy *OneForOneStrategy) Handle(child interfaces.ActorRef, reason interface{}) interfaces.Directive {
	return strategy.decider(child, reason)
}

func NewOneForOneStrategy(maxNrOfRetries int, withinTimeRangeMilliseconds int, decider interfaces.Decider) interfaces.SupervisionStrategy {
	return &OneForOneStrategy{
        maxNrOfRetries: maxNrOfRetries,
        withinTimeRangeMilliseconds: withinTimeRangeMilliseconds,
        decider: decider,
    }
}

func DefaultDecider (child interfaces.ActorRef, reason interface{}) interfaces.Directive {
    return interfaces.Restart
}

var defaultStrategy interfaces.SupervisionStrategy = NewOneForOneStrategy(10,30000,DefaultDecider)
func DefaultStrategy() interfaces.SupervisionStrategy {
    return defaultStrategy
}