package cluster

import (
	"sort"
	"strconv"
	"strings"

	murmur32 "github.com/twmb/murmur3"
)

type Members []*Member

func (m *Members) ToSet() *MemberSet {
	return NewMemberSet(*m)
}

func (m *Member) HasKind(kind string) bool {
	for _, k := range m.Kinds {
		if k == kind {
			return true
		}
	}

	return false
}

// Address return a "host:port".
// Member defined by protos.proto
func (m *Member) Address() string {
	return m.Host + ":" + strconv.FormatInt(int64(m.Port), 10)
}

func TopologyHash(members Members) uint64 {
	// C# version
	// var x = membersByMemberId.Select(m => m.Id).OrderBy(i => i).ToArray();
	// var key = string.Join("", x);
	// var hashBytes = MurmurHash2.Hash(key);
	// return hashBytes;

	sort.Slice(members, func(i, j int) bool {
		return members[i].Id < members[j].Id
	})

	// I assume this is not the fastest way to do this?
	s := ""
	for _, m := range members {
		s += m.Id
	}

	// TODO: this HAS to be compatible with the same hashBytes in .NET
	// add plenty of tests
	hash := murmur32.Sum64([]byte(s))

	return hash
}

func MembersToMap(members Members) map[string]*Member {
	mapp := make(map[string]*Member)
	for _, m := range members {
		mapp[m.Id] = m
	}

	return mapp
}

func SortMembers(members Members) {
	sort.Slice(members, func(i, j int) bool {
		addrI := members[i].Id
		addrJ := members[j].Id

		return strings.Compare(addrI, addrJ) > 0
	})
}

func CopySortMembers(members Members) Members {
	tmp := append(make(Members, 0, len(members)), members...)
	SortMembers(tmp)

	return tmp
}
