package actor

import (
	"sync"
	"testing"
)

type dummyMessageBatch struct {
	messages []interface{}
}

func (d dummyMessageBatch) GetMessages() []interface{} {
	return d.messages
}

func TestActorReceivesEachMessageInAMessageBatch(t *testing.T) {
	// each message in the batch
	seenMessagesWg := sync.WaitGroup{}
	seenMessagesWg.Add(10)

	// the batch message itself
	seenBatchMessageWg := sync.WaitGroup{}
	seenBatchMessageWg.Add(1)

	pid := rootContext.Spawn(PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(*DummyMessage); ok {
			seenMessagesWg.Done()
		}

		if _, ok := ctx.Message().(*dummyMessageBatch); ok {
			seenBatchMessageWg.Done()
		}
	}))

	batch := &dummyMessageBatch{messages: make([]interface{}, 10)}

	for i := 0; i < 10; i++ {
		batch.messages[i] = &DummyMessage{}
	}

	rootContext.Send(pid, batch)

	seenMessagesWg.Wait()
	seenBatchMessageWg.Wait()
}
