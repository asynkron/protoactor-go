package cluster

import "time"

//getMembers lists all known, reachable and unreachable members for this kind
//TODO: this needs to be implemented,we could send a `Request` to the membership actor, but this seems flaky.
//a threadsafe map would be better
func getMembers(kind string) []string {
	var members []string

	for {
		res, err := memberlistPID.RequestFuture(&MemberByKindRequest{kind: kind, onlyAlive: true}, 5*time.Second).Result()
		if err == nil {
			t, ok := res.(*MemberByKindResponse)
			if ok && len(t.members) > 0 {
				members = t.members
				break
			}
		}
		time.Sleep(time.Millisecond * 500)
	}

	return members
}

type MemberByKindRequest struct {
	kind      string
	onlyAlive bool
}

type MemberByKindResponse struct {
	members []string
}
