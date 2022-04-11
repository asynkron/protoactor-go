package actor

import (
	"testing"
)

type EsTestMsg struct{}

func TestSendsMessagesToEventStream(t *testing.T) {
	testCases := []struct {
		name    string
		message interface{}
	}{
		{name: "plain", message: &EsTestMsg{}},
		{name: "envelope", message: WrapEnvelope(&EsTestMsg{})},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			system := NewActorSystem()

			gotMessageChan := make(chan struct{}, 1)

			subscription := system.EventStream.Subscribe(func(evt interface{}) {
				if _, ok := evt.(*EsTestMsg); ok {
					gotMessageChan <- struct{}{}
				}
			})
			defer system.EventStream.Unsubscribe(subscription)

			pid := system.NewLocalPID("eventstream")

			system.Root.Send(pid, testCase.message)

			<-gotMessageChan
		})
	}
}
