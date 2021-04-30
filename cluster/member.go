package cluster

import (
	murmur32 "github.com/spaolacci/murmur3"
	"sort"
	"strconv"
)

// Address return a "host:port".
// Member defined by protos.proto
func (m *Member) Address() string {
	return m.Host + ":" + strconv.FormatInt(int64(m.Port), 10)
}

func GetMembershipHashCode(members []Member) uint32 {

	//C# version
	//var x = members.Select(m => m.Id).OrderBy(i => i).ToArray();
	//var key = string.Join("", x);
	//var hash = MurmurHash2.Hash(key);
	//return hash;

	sort.Slice(members, func(i, j int) bool {
		return members[i].Id < members[j].Id
	})

	//I assume this is not the fastest way to do this?
	s := ""
	for _, m := range members {
		s += m.Id
	}

	//TODO: this HAS to be compatible with the same hash in .NET
	//add plenty of tests
	hash := murmur32.Sum32([]byte(s))
	return hash
}
