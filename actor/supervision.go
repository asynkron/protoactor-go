package actor

import (
	"github.com/rogeralsing/goactor/interfaces"
)

type OneForOneStrategy struct {
	maxNrOfRetries              int
	withinTimeRangeMilliseconds int
	decider                     interfaces.Decider
}

func (strategy *OneForOneStrategy) Handle(child interfaces.ActorRef, cause interface{}) interfaces.Directive {
	return strategy.decider(child, cause)
}

func NewOneForOneStrategy(maxNrOfRetries int, withinTimeRangeMilliseconds int, decider interfaces.Decider) interfaces.SupervisionStrategy {
	return &OneForOneStrategy{
        maxNrOfRetries: maxNrOfRetries,
        withinTimeRangeMilliseconds: withinTimeRangeMilliseconds,
        decider: decider,
    }
}
