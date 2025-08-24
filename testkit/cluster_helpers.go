package testkit

import (
	"context"
	"fmt"
	"time"

	"github.com/asynkron/protoactor-go/cluster"
)

// ExpectMemberToExist waits until the cluster's member list contains the specified member.
// A default timeout of 10 seconds is used when timeout is zero.
func ExpectMemberToExist(c *cluster.Cluster, member *cluster.Member, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return AwaitCondition(context.Background(), func() bool {
		return c.MemberList.ContainsMemberID(member.Id)
	}, timeout, fmt.Sprintf("Member %s was not found within %v", member.Id, timeout))
}
