package consul_cluster

import (
	"log"
	"testing"
	"time"
)

func TestRegisterMember(t *testing.T) {
	p := New()
	defer p.Shutdown()
	err := p.RegisterMember("mycluster", "127.0.0.1", 8000, []string{"a", "b"})
	if err != nil {
		log.Fatal(err)
	}
}

func TestReRegisterMember(t *testing.T) {
	p := New()
	defer p.Shutdown()
	err := p.RegisterMember("mycluster", "127.0.0.1", 8000, []string{"a", "b"})
	if err != nil {
		log.Fatal(err)
	}
	time.Sleep(60 * time.Second)
}
