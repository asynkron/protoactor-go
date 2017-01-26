package cluster

import "time"

//getMembers lists all known, reachable and unreachable members for this kind
//TODO: this needs to be implemented,we could send a `Request` to the membership actor, but this seems flaky.
//a threadsafe map would be better
func getMembers(kind string) []string {
	res, err := memberlistPID.RequestFuture(&MemberByKindRequest{kind: kind, onlyAlive: true}, 5*time.Second).Result()
	if err != nil {
		//TODO: lets say a node asks for an actor of kind X, which is not registered on the local node
		//and no other nodes are currently avaialbe, what should be the behavior?
		panic("No members found")
	}
	t, ok := res.(*MemberByKindResponse)
	if !ok {
		plog.Error("Failed to cast members by kind response")
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
