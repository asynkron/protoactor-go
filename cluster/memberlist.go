package cluster

import (
	"log"
	"time"
)

//getMembers lists all known, reachable and unreachable members for this kind
//TODO: this needs to be implemented,we could send a `Request` to the membership actor, but this seems flaky.
//a threadsafe map would be better
func getMembers(kind string) []string {
	res, err := memberlistPID.RequestFuture(&MemberByKindRequest{kind: kind}, 5*time.Second).Result()
	if err != nil {
		log.Printf("Failed to get members by kind")
		return nil
	}
	t, ok := res.(*MemberByKindResponse)
	if !ok {
		log.Printf("Failed to cast members by kind response")
		return nil
	}

	return t.members
}

type MemberByKindRequest struct {
	kind      string
	onlyAlive bool
}

type MemberByKindResponse struct {
	members []string
}
