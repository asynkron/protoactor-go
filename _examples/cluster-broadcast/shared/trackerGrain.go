package shared

import (
	"fmt"
	"github.com/asynkron/protoactor-go/cluster"
	"strings"
)

type TrackGrain struct {
	cluster.Grain
	grainsMap map[string]bool
}

func (t *TrackGrain) Init(ci *cluster.ClusterIdentity, c *cluster.Cluster) {
	t.Grain.Init(ci, c)
	t.grainsMap = map[string]bool{}
}

func (t *TrackGrain) Terminate() {
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
	for grainAddress, _ := range t.grainsMap {
		calcGrain := GetCalculatorGrainClient(t.Cluster(), grainAddress)
		grainTotal, err := calcGrain.GetCurrent(&Noop{})
		if err != nil {
			fmt.Sprintf("Grain %s issued an error : %s", grainAddress, err)
		}
		fmt.Sprintf("Grain %s - %v", grainAddress, grainTotal.Number)
		totals[grainAddress] = grainTotal.Number
	}

	return &TotalsResponse{Totals: totals}, nil
}
