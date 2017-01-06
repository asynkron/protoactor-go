package consul_cluster

import (
	"log"
	"testing"
)

func TestRegisterMember(t *testing.T) {
	p := New()
	defer p.Shutdown()
	err := p.RegisterMember("mycluster", "127.0.0.1", 8000, []string{"a", "b"})
	if err != nil {
		log.Fatal(err)
	}
}
