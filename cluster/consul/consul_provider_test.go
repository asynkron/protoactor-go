package consul

import (
	"log"
	"strconv"
	"sync"
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

func TestUpdateMemberStatusValueDoesNotReregisterAfterShutdown(t *testing.T) {
	if testing.Short() {
		return
	}

	p, _ := New()

	clusterName := "mycluster4"
	port := 8001

	err := p.RegisterMember(clusterName, "127.0.0.1", port, []string{"a", "b"}, &TestMemberStatusValue{value: 0}, &TestMemberStatusValueSerializer{})
	if err != nil {
		t.Error(err)
	}

	found, _ := findService(t, p, clusterName, port)

	if !found {
		log.Fatal("service was not registered in consul")
	}

	err = p.Shutdown()
	if err != nil {
		t.Error(err)
	}

	newStatusValue := &TestMemberStatusValue{value: 3}
	err = p.UpdateMemberStatusValue(newStatusValue)
	if err == nil {
		log.Fatal("Expected error since service should not re-register after shutdown was initialized")
	} else if err != ProviderShuttingDownError {
		t.Error(err)
	}

	found, status := findService(t, p, clusterName, port)

	if found {
		log.Fatalf("service was re-registered in consul after shutdown (status: %s)", status)
	}
}

func TestUpdateTTLDoesNotReregisterAfterShutdown(t *testing.T) {
	if testing.Short() {
		return
	}

	clusterName := "mycluster5"
	port := 8001

	originalBlockingUpdateTTLFunc := blockingUpdateTTLFunc
	defer func() {
		blockingUpdateTTLFunc = originalBlockingUpdateTTLFunc
	}()

	registeredInConsul := false

	var blockingUpdateTTLBlockReachedWg sync.WaitGroup
	blockingUpdateTTLBlockReachedWg.Add(1)

	var rw sync.RWMutex
	shutdownShouldHaveResolved := false

	// this simulates `blockingUpdateTTL` in `UpdateTTL` to be slower than `Shutdown`
	blockingUpdateTTLFunc = func(p *ConsulProvider) error {
		// default behaviour until `RegisterMember` was called
		if !registeredInConsul || p.port != port {
			return originalBlockingUpdateTTLFunc(p)
		}

		blockingUpdateTTLBlockReachedWg.Done()

		// wait until it is safe to assume that `Shutdown` will not finish until this call resolves or that `Shutdown` is already done
		for {
			rw.RLock()
			if shutdownShouldHaveResolved {
				rw.RUnlock()
				break
			}
			rw.RUnlock()
			time.Sleep(10 * time.Millisecond)
		}
		return originalBlockingUpdateTTLFunc(p)
	}

	p, _ := New()

	err := p.RegisterMember(clusterName, "127.0.0.1", port, []string{"a", "b"}, &TestMemberStatusValue{value: 0}, &TestMemberStatusValueSerializer{})
	if err != nil {
		t.Error(err)
	}
	registeredInConsul = true

	found, _ := findService(t, p, clusterName, port)

	if !found {
		log.Fatal("service was not registered in consul")
	}

	// Wait until `blockingUpdateTTL` waits for the deregistration/shutdown of the member
	blockingUpdateTTLBlockReachedWg.Wait()

	go func() {
		// if after 5 seconds `Shutdown` did not resolve, assume that it will not resolve until `blockingUpdateTTL` resolves
		time.Sleep(5 * time.Second)
		rw.Lock()
		shutdownShouldHaveResolved = true
		rw.Unlock()
	}()

	err = p.Shutdown()
	if err != nil {
		t.Error(err)
	}
	rw.Lock()
	shutdownShouldHaveResolved = true
	rw.Unlock()

	// since `UpdateTTL` runs in a separate goroutine we need to wait until it is actually finished before checking the member's clusterstatus
	p.updateTTLWaitGroup.Wait()

	found, status := findService(t, p, clusterName, port)
	if found {
		t.Fatalf("service was still registered in consul after shutdown (service status: %s)", status)
	}
}

func findService(t *testing.T, p *ConsulProvider, service string, port int) (found bool, status string) {
	entries, _, err := p.client.Health().Service(service, "", false, nil)
	if err != nil {
		t.Error(err)
	}

	for _, entry := range entries {
		if entry.Service.Port == port {
			return true, entry.Checks.AggregatedStatus()
		}
	}
	return false, ""
}
