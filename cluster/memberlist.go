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
		if t, ok := res.(*MembersByKindResponse); ok && len(t.members) > 0 {
			return t.members
		}
	}
	return nil
}

func getMemberByDHT(name, kind string) string {
	res, err := memberlistPID.RequestFuture(&MemberByDHTRequest{name, kind}, 5*time.Second).Result()
	if err == nil {
		if t, ok := res.(*MemberByDHTResponse); ok {
			return t.member
		}
	}
	return ""
}

type MembersByKindRequest struct {
	kind      string
	onlyAlive bool
}

type MembersByKindResponse struct {
	members []string
}

type MemberByDHTRequest struct {
	name string
	kind string
}

type MemberByDHTResponse struct {
	member string
}
