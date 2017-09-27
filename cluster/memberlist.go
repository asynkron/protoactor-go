package cluster

import (
	"time"

	"github.com/AsynkronIT/protoactor-go/cluster/rendezvous"
	"github.com/AsynkronIT/protoactor-go/log"
)

//getMembers lists all known, reachable and unreachable members for this kind
//TODO: this needs to be implemented,we could send a `Request` to the membership actor, but this seems flaky.
//a threadsafe map would be better
func getMembers(kind string) []string {
	res, err := memberlistPID.RequestFuture(&MemberByKindRequest{kind: kind, onlyAlive: true}, 5*time.Second).Result()
	if err == nil {
		if t, ok := res.(*MemberByKindResponse); ok && len(t.members) > 0 {
			return t.members
		}
	}
	return nil
}

func getMember(name, kind string) string {
	members := getMembers(kind)
	if members == nil {
		plog.Error("getMember: failed to get member", log.String("kind", kind))
		return ""
	}

	//Can be optimized by caching members list, instead of hashing them everytime.
	rdv := rendezvous.New(members...)
	return rdv.Get(name)
}

type MemberByKindRequest struct {
	kind      string
	onlyAlive bool
}

type MemberByKindResponse struct {
	members []string
}
