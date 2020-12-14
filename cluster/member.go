package cluster

import "strconv"

// Address return a "host:port".
// Member defined by protos.proto
func (m *Member) Address() string {
	return m.Host + ":" + strconv.FormatInt(int64(m.Port), 10)
}
