package shared

import (
	"fmt"
	"github.com/otherview/protoactor-go/cluster"
)

type TrackGrain struct {
	cluster.Grain
	grainsMap map[string]bool
}

func (t *TrackGrain) Init(id string)  {
	t.Grain.Init(id)
	t.grainsMap = map[string]bool{}
}

func (t *TrackGrain) Terminate()  {
}

func (t *TrackGrain) RegisterGrain(n *RegisterMessage, ctx cluster.GrainContext) (*Noop, error) {
	t.grainsMap[n.GrainId] = true
	return &Noop{}, nil
}

func (t *TrackGrain) DeregisterGrain(n *RegisterMessage, ctx cluster.GrainContext) (*Noop, error) {
	delete(t.grainsMap, n.GrainId)
	return &Noop{}, nil
}

func (t *TrackGrain) BroadcastGetCounts(n *Noop, ctx cluster.GrainContext) (*TotalsResponse, error) {

	totals := map[string]int64{}
	for grainAddress, _ := range t.grainsMap {
		calcGrain := GetCalculatorGrain(grainAddress)
		grainTotal, err := calcGrain.GetCurrent(&Noop{})
		if err != nil {
			fmt.Sprintf("Grain %s issued an error : %s", grainAddress, err)
		}
		fmt.Sprintf("Grain %s - %v", grainAddress, grainTotal.Number)
		totals[grainAddress] = grainTotal.Number
	}

	return &TotalsResponse{Totals: totals}, nil
}