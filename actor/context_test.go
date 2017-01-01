package actor

import "testing"

func BenchmarkActorCell_Next(b *testing.B) {
	ac := &actorCell{actor: nullReceive}
	ac.Become(nullReceive.Receive)
	for i := 0; i < b.N; i++ {
		ac.Next()
	}
}
