package cluster

import (
	murmur32 "github.com/spaolacci/murmur3"
	"sort"
	"strconv"
	"strings"
)

type Members = []*Member

// Address return a "host:port".
// Member defined by protos.proto
func (m *Member) Address() string {
	return m.Host + ":" + strconv.FormatInt(int64(m.Port), 10)
}

func TopologyHash(members Members) uint64 {

	//C# version
	//var x = membersByMemberId.Select(m => m.Id).OrderBy(i => i).ToArray();
	//var key = string.Join("", x);
	//var hashBytes = MurmurHash2.Hash(key);
	//return hashBytes;

	sort.Slice(members, func(i, j int) bool {
		return members[i].Id < members[j].Id
	})

	//I assume this is not the fastest way to do this?
	s := ""
	for _, m := range members {
		s += m.Id
	}

	//TODO: this HAS to be compatible with the same hashBytes in .NET
	//add plenty of tests
	hash := murmur32.Sum64([]byte(s))
	return hash
}

func MembersToSet(members Members) map[string]bool {
	set := make(map[string]bool)
	for _, m := range members {
		set[m.Id] = true
	}
	return set
}

func MembersToMap(members Members) map[string]*Member {
	mapp := make(map[string]*Member)
	for _, m := range members {
		mapp[m.Id] = m
	}
	return mapp
}

func MembersExcept(members Members, except map[string]bool) Members {
	filtered := make(Members, 0)
	for _, m := range members {
		if _, found := except[m.Id]; found {
			continue
		}
		filtered = append(filtered, m)
	}
	return filtered
}

func AddMembersToSet(set map[string]bool, members Members) {
	for _, m := range members {
		set[m.Id] = true
	}
}

func MemberIds(members Members) []string {
	ids := make([]string, 0)
	for _, m := range members {
		ids = append(ids, m.Id)
	}
	return ids
}

func GroupMembersByKind(members Members) map[string]Members {
	groups := map[string]Members{}
	for _, member := range members {
		for _, kind := range member.Kinds {
			if list, ok := groups[kind]; ok {
				groups[kind] = append(list, member)
			} else {
				groups[kind] = Members{member}
			}
		}
	}
	return groups
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

func buildSortedMembers(m map[string]*Member) Members {
	list := make(Members, len(m))
	i := 0
	for _, member := range m {
		list[i] = member
		i++
	}
	SortMembers(list)
	return list
}
