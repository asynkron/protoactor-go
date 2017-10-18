package cluster

import (
	"time"
)

//getMembers lists all known, reachable and unreachable members for this kind
//TODO: this needs to be implemented,we could send a `Request` to the membership actor, but this seems flaky.
//a threadsafe map would be better
func getMembers(kind string) []string {
	res, err := memberlistPID.RequestFuture(&MembersByKindRequest{kind: kind, onlyAlive: true}, 5*time.Second).Result()
	if err == nil {
		if t, ok := res.(*MembersResponse); ok && len(t.members) > 0 {
			return t.members
		}
	}
	return nil
}

func getPartitionMember(name, kind string) string {
	res, err := memberlistPID.RequestFuture(&PartitionMemberRequest{name, kind}, 5*time.Second).Result()
	if err == nil {
		if t, ok := res.(*MemberResponse); ok {
			return t.member
		}
	}
	return ""
}

func getActivatorMember(kind string) string {
	res, err := memberlistPID.RequestFuture(&ActivatorMemberRequest{kind}, 5*time.Second).Result()
	if err == nil {
		if t, ok := res.(*MemberResponse); ok {
			return t.member
		}
	}
	return ""
}

type MembersByKindRequest struct {
	kind      string
	onlyAlive bool
}

type PartitionMemberRequest struct {
	name string
	kind string
}

type ActivatorMemberRequest struct {
	kind string
}

type MemberResponse struct {
	member string
}

type MembersResponse struct {
	members []string
}
