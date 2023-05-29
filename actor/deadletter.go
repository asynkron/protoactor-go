package actor

import (
	"context"
	"fmt"
	"strings"

	"github.com/asynkron/protoactor-go/log"
	"github.com/asynkron/protoactor-go/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type deadLetterProcess struct {
	actorSystem *ActorSystem
}

var _ Process = &deadLetterProcess{}

func NewDeadLetter(actorSystem *ActorSystem) *deadLetterProcess {
	dp := &deadLetterProcess{
		actorSystem: actorSystem,
	}

	shouldThrottle := NewThrottle(actorSystem.Config.DeadLetterThrottleCount, actorSystem.Config.DeadLetterThrottleInterval, func(i int32) {
		plog.Info("[DeadLetter]", log.Int64("throttled", int64(i)))
	})

	actorSystem.ProcessRegistry.Add(dp, "deadletter")
	_ = actorSystem.EventStream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetterEvent); ok {

			// send back a response instead of timeout.
			if deadLetter.Sender != nil {
				actorSystem.Root.Send(deadLetter.Sender, &DeadLetterResponse{})
			}

			// bail out if sender is set and deadletter request logging is false
			if !actorSystem.Config.DeadLetterRequestLogging && deadLetter.Sender != nil {
				return
			}

			if _, isIgnoreDeadLetter := deadLetter.Message.(IgnoreDeadLetterLogging); !isIgnoreDeadLetter {
				if shouldThrottle() == Open {
					plog.Debug("[DeadLetter]", log.Stringer("pid", deadLetter.PID), log.TypeOf("msg", deadLetter.Message), log.Stringer("sender", deadLetter.Sender))
				}
			}
		}
	})

	// this subscriber may not be deactivated.
	// it ensures that Watch commands that reach a stopped actor gets a Terminated message back.
	// This can happen if one actor tries to Watch a PID, while another thread sends a Stop message.
	actorSystem.EventStream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetterEvent); ok {
			if m, ok := deadLetter.Message.(*Watch); ok {
				// we know that this is a local actor since we get it on our own event stream, thus the address is not terminated
				m.Watcher.sendSystemMessage(actorSystem, &Terminated{
					Who: deadLetter.PID,
					Why: TerminatedReason_NotFound,
				})
			}
		}
	})

	return dp
}

// A DeadLetterEvent is published via event.Publish when a message is sent to a nonexistent PID
type DeadLetterEvent struct {
	PID     *PID        // The invalid process, to which the message was sent
	Message interface{} // The message that could not be delivered
	Sender  *PID        // the process that sent the Message
}

func (dp *deadLetterProcess) SendUserMessage(pid *PID, message interface{}) {
	metricsSystem, ok := dp.actorSystem.Extensions.Get(extensionId).(*Metrics)
	if ok && metricsSystem.enabled {
		ctx := context.Background()
		if instruments := metricsSystem.metrics.Get(metrics.InternalActorMetrics); instruments != nil {
			labels := []attribute.KeyValue{
				attribute.String("address", dp.actorSystem.Address()),
				attribute.String("messagetype", strings.Replace(fmt.Sprintf("%T", message), "*", "", 1)),
			}

			instruments.DeadLetterCount.Add(ctx, 1, metric.WithAttributes(labels...))
		}
	}
	_, msg, sender := UnwrapEnvelope(message)
	dp.actorSystem.EventStream.Publish(&DeadLetterEvent{
		PID:     pid,
		Message: msg,
		Sender:  sender,
	})
}

func (dp *deadLetterProcess) SendSystemMessage(pid *PID, message interface{}) {
	dp.actorSystem.EventStream.Publish(&DeadLetterEvent{
		PID:     pid,
		Message: message,
	})
}

func (dp *deadLetterProcess) Stop(pid *PID) {
	dp.SendSystemMessage(pid, stopMessage)
}
