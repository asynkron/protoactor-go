package testkit

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/remote"
)

func TestExpectMemberToExist(t *testing.T) {
	sys := actor.NewActorSystem()
	c := &cluster.Cluster{ActorSystem: sys}
	c.Remote = remote.NewRemote(sys, remote.Configure("127.0.0.1", 0))
	c.MemberList = cluster.NewMemberList(c)

	member := &cluster.Member{Id: "node1"}
	members := cluster.Members{member}
	c.MemberList.UpdateClusterTopology(members)

	if err := ExpectMemberToExist(c.MemberList, member.Id, time.Second); err != nil {
		t.Fatalf("expected member to exist: %v", err)
	}

	if err := ExpectMemberToExist(c.MemberList, "missing", 50*time.Millisecond); err == nil {
		t.Fatalf("expected error for missing member")
	}
}
