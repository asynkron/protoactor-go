package shared

import (
	"fmt"
	"strings"

	"github.com/asynkron/protoactor-go/cluster"
)

type TrackGrain struct {
	grainsMap map[string]bool
}

func (t *TrackGrain) ReceiveDefault(ctx cluster.GrainContext) {
}

func (t *TrackGrain) Init(ctx cluster.GrainContext) {
	t.grainsMap = map[string]bool{}
}

func (t *TrackGrain) Terminate(ctx cluster.GrainContext) {
}

func (t *TrackGrain) RegisterGrain(n *RegisterMessage, ctx cluster.GrainContext) (*Noop, error) {
	parts := strings.Split(n.GrainId, "/")
	grainID := parts[len(parts)-1]
	t.grainsMap[grainID] = true
	return &Noop{}, nil
}

func (t *TrackGrain) DeregisterGrain(n *RegisterMessage, ctx cluster.GrainContext) (*Noop, error) {
	delete(t.grainsMap, n.GrainId)
	return &Noop{}, nil
}

func (t *TrackGrain) BroadcastGetCounts(n *Noop, ctx cluster.GrainContext) (*TotalsResponse, error) {
	totals := map[string]int64{}
	for grainAddress := range t.grainsMap {
		calcGrain := GetCalculatorGrainClient(ctx.Cluster(), grainAddress)
		grainTotal, err := calcGrain.GetCurrent(&Noop{})
		if err != nil {
			fmt.Sprintf("Grain %s issued an error : %s", grainAddress, err)
		}
		fmt.Sprintf("Grain %s - %v", grainAddress, grainTotal.Number)
		totals[grainAddress] = grainTotal.Number
	}

	return &TotalsResponse{Totals: totals}, nil
}
