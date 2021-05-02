package cluster

import (
	murmur32 "github.com/spaolacci/murmur3"
	"sort"
	"strconv"
	"strings"
)

// Address return a "host:port".
// Member defined by protos.proto
func (m *Member) Address() string {
	return m.Host + ":" + strconv.FormatInt(int64(m.Port), 10)
}

func TopologyHash(members []*Member) uint64 {

	//C# version
	//var x = membersByMemberId.Select(m => m.Id).OrderBy(i => i).ToArray();
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
	hash := murmur32.Sum64([]byte(s))
	return hash
}

func MembersToSet(members []*Member) map[string]bool {
	set := make(map[string]bool)
	for _, m := range members {
		set[m.Id] = true
	}
	return set
}

func MembersToMap(members []*Member) map[string]*Member {
	mapp := make(map[string]*Member)
	for _, m := range members {
		mapp[m.Id] = m
	}
	return mapp
}

func MembersExcept(members []*Member, except map[string]bool) []*Member {
	filtered := make([]*Member, 0)
	for _, m := range members {
		if _, found := except[m.Id]; found {
			continue
		}
		filtered = append(filtered, m)
	}
	return filtered
}

func AddMembersToSet(set map[string]bool, members []*Member) {
	for _, m := range members {
		set[m.Id] = true
	}
}

func MemberIds(members []*Member) []string {
	ids := make([]string, 0)
	for _, m := range members {
		ids = append(ids, m.Id)
	}
	return ids
}

func GroupMembersByKind(members []*Member) map[string][]*Member {
	groups := map[string][]*Member{}
	for _, member := range members {
		for _, kind := range member.Kinds {
			if list, ok := groups[kind]; ok {
				groups[kind] = append(list, member)
			} else {
				groups[kind] = []*Member{member}
			}
		}
	}
	return groups
}

func SortMembers(members []*Member) {
	sort.Slice(members, func(i, j int) bool {
		addrI := members[i].Id
		addrJ := members[j].Id
		return strings.Compare(addrI, addrJ) > 0
	})
}

func buildSortedMembers(m map[string]*Member) []*Member {
	list := make([]*Member, len(m))
	i := 0
	for _, member := range m {
		list[i] = member
		i++
	}
	SortMembers(list)
	return list
}
