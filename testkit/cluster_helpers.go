package testkit

import (
	"context"
	"fmt"
	"time"
)

// memberLister is the minimal interface needed from a cluster's member list.
// It allows ExpectMemberToExist to be used without importing the cluster package,
// avoiding import cycles with packages that also use the testkit.
type memberLister interface {
	ContainsMemberID(memberID string) bool
}

// ExpectMemberToExist waits until the supplied member list reports the given member ID.
// A default timeout of 10 seconds is used when timeout is zero.
func ExpectMemberToExist(ml memberLister, memberID string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return AwaitCondition(context.Background(), func() bool {
		return ml.ContainsMemberID(memberID)
	}, timeout, fmt.Sprintf("Member %s was not found within %v", memberID, timeout))
}
