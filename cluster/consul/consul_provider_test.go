package consul

import (
	"log"
	"testing"
	"time"

	"github.com/otherview/protoactor-go/cluster"
	"github.com/otherview/protoactor-go/eventstream"
)

func TestRegisterMember(t *testing.T) {
	if testing.Short() {
		return
	}

	p, _ := New()
	defer p.Shutdown()
	err := p.RegisterMember("mycluster", "127.0.0.1", 8000, []string{"a", "b"}, nil, &cluster.NilMemberStatusValueSerializer{})
	if err != nil {
		log.Fatal(err)
	}
}

func TestRefreshMemberTTL(t *testing.T) {
	if testing.Short() {
		return
	}

	p, _ := New()
	defer p.Shutdown()
	err := p.RegisterMember("mycluster", "127.0.0.1", 8000, []string{"a", "b"}, nil, &cluster.NilMemberStatusValueSerializer{})
	if err != nil {
		log.Fatal(err)
	}
	p.MonitorMemberStatusChanges()
	eventstream.Subscribe(func(m interface{}) {
		log.Printf("Event %+v", m)
	})
	time.Sleep(60 * time.Second)
}

func TestRegisterMultipleMembers(t *testing.T) {
	if testing.Short() {
		return
	}

	members := []struct{
		cluster string
		address string
		port int
	} {
		{"mycluster2", "127.0.0.1", 8001 },
		{"mycluster2", "127.0.0.1", 8002 },
		{"mycluster2", "127.0.0.1", 8003 },
	}

	p, _ := New()
	defer p.Shutdown()

	for _, member := range members {
		err := p.RegisterMember(member.cluster, member.address, member.port, []string{"a", "b"}, nil, &cluster.NilMemberStatusValueSerializer{})
		if err != nil {
			log.Fatal(err)
		}
	}

	entries, _, err := p.client.Health().Service("mycluster2", "", true, nil)
	if err != nil {
		log.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		found = false
		for _, member := range members {
			if entry.Service.Port == member.port {
				found = true
			}
		}
		if !found {
			t.Errorf("Member port not found - ID:%v Address: %v:%v \n", entry.Service.ID, entry.Service.Address, entry.Service.Port)
		}
	}
}
