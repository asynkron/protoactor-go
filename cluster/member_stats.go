package cluster

// MemberStats contains grain statistics for a cluster member.
type MemberStats struct {
	MemberID   string
	Address    string
	GrainCount int64
	ByKind     map[string]int64
}

// MemberStats returns grain statistics for all cluster members by reading
// the existing gossiped heartbeat data.
func (c *Cluster) MemberStats() ([]MemberStats, error) {
	state, err := c.Gossip.GetState(HeartbeatKey)
	if err != nil {
		return nil, err
	}

	members := c.MemberList.Members()
	memberMap := make(map[string]*Member)
	if members != nil {
		for _, m := range members.Members() {
			memberMap[m.Id] = m
		}
	}

	var result []MemberStats
	for memberID, gkv := range state {
		if gkv.Value == nil {
			continue
		}

		var hb MemberHeartbeat
		if err := gkv.Value.UnmarshalTo(&hb); err != nil {
			continue
		}

		stats := MemberStats{
			MemberID: memberID,
			ByKind:   make(map[string]int64),
		}

		if m, ok := memberMap[memberID]; ok {
			stats.Address = m.Address()
		}

		if hb.ActorStatistics != nil && hb.ActorStatistics.ActorCount != nil {
			for kind, count := range hb.ActorStatistics.ActorCount {
				stats.ByKind[kind] = count
				stats.GrainCount += count
			}
		}

		result = append(result, stats)
	}

	return result, nil
}
