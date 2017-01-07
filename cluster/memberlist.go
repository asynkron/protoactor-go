package cluster

var (
	localMember string
)

//getMembers lists all known, reachable and unreachable members for this kind
//TODO: this needs to be implemented,we could send a `Request` to the membership actor, but this seems flaky.
//a threadsafe map would be better
func getMembers(kind string) []string {
	return nil
}
