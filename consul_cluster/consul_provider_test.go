package consul_cluster

import (
	"log"
	"testing"
	"time"
)

func TestRegisterMember(t *testing.T) {
	if testing.Short() {
		return
	}

	p, _ := New()
	defer p.Shutdown()
	err := p.RegisterMember("mycluster", "127.0.0.1", 8000, []string{"a", "b"})
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
	err := p.RegisterMember("mycluster", "127.0.0.1", 8000, []string{"a", "b"})
	if err != nil {
		log.Fatal(err)
	}
	c := p.GetStatusChanges()
	go func() {
		for {
			s := <-c
			log.Printf("Cluster status %v:%v, %v, %v", s.Address, s.Port, s.Kinds, s.Alive)
		}
	}()
	time.Sleep(60 * time.Second)
}
