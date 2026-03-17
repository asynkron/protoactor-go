package cluster

// ValidateActivationMember checks whether a member ID is still present in
// the cluster's current topology. Used by identity lookups to detect stale
// activation records left by departed members.
//
// This matches the .NET IdentityStorageWorker.ValidateAndMapToPid() pattern:
// before returning a PID from a stored activation, verify the owning member
// is still alive.
func ValidateActivationMember(ml *MemberList, memberID string) bool {
	return ml.ContainsMemberID(memberID)
}
