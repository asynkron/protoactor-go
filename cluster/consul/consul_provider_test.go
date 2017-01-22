package consul

import (
	"log"
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/eventstream"
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
	p.MonitorMemberStatusChanges()
	eventstream.Subscribe(func(m interface{}) {
		log.Printf("Event %+v", m)
	})
	time.Sleep(60 * time.Second)
}
