package consul

import (
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/eventstream"
)

func TestRegisterMember(t *testing.T) {
	if testing.Short() {
		return
	}

	p, _ := New()
	defer p.Shutdown()
	err := p.RegisterMember("mycluster", "127.0.0.1", 8000, []string{"a", "b"}, &TestMemberStatusValue{value: 0}, &TestMemberStatusValueSerializer{})
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
	err := p.RegisterMember("mycluster", "127.0.0.1", 8000, []string{"a", "b"}, &TestMemberStatusValue{value: 0}, &TestMemberStatusValueSerializer{})
	if err != nil {
		log.Fatal(err)
	}
	p.MonitorMemberStatusChanges()
	eventstream.Subscribe(func(m interface{}) {
		log.Printf("Event %+v", m)
	})
	time.Sleep(20 * time.Second)
}

func TestRegisterMultipleMembers(t *testing.T) {
	if testing.Short() {
		return
	}

	members := []struct {
		cluster string
		address string
		port    int
	}{
		{"mycluster2", "127.0.0.1", 8001},
		{"mycluster2", "127.0.0.1", 8002},
		{"mycluster2", "127.0.0.1", 8003},
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

func TestUpdateMemberStatusValue(t *testing.T) {
	if testing.Short() {
		return
	}

	p, _ := New()
	defer p.Shutdown()

	err := p.RegisterMember("mycluster3", "127.0.0.1", 8001, []string{"a", "b"}, &TestMemberStatusValue{value: 0}, &TestMemberStatusValueSerializer{})
	if err != nil {
		log.Fatal(err)
	}

	newStatusValue := &TestMemberStatusValue{value: 3}
	err = p.UpdateMemberStatusValue(newStatusValue)
	if err != nil {
		t.Error(err)
	}

	entries, _, err := p.client.Health().Service("mycluster3", "", true, nil)
	if err != nil {
		log.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		sv := p.statusValueSerializer.Deserialize(entry.Service.Meta["StatusValue"])
		if sv.IsSame(newStatusValue) {
			found = true
		}
	}
	if !found {
		t.Errorf("Member status value not found")
	}
}

type TestMemberStatusValue struct{ value int }

func (v *TestMemberStatusValue) IsSame(val cluster.MemberStatusValue) bool {
	if val == nil {
		return false
	}
	if sv, ok := val.(*TestMemberStatusValue); ok {
		return sv.value == v.value
	}
	return false
}

type TestMemberStatusValueSerializer struct{}

func (s *TestMemberStatusValueSerializer) Serialize(val cluster.MemberStatusValue) string {
	dVal, _ := val.(*TestMemberStatusValue)
	return strconv.Itoa(dVal.value)
}

func (s *TestMemberStatusValueSerializer) Deserialize(val string) cluster.MemberStatusValue {
	weight, _ := strconv.Atoi(val)
	return &TestMemberStatusValue{value: weight}
}
