package actor

import (
	"context"
	"log/slog"

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
		actorSystem.Logger().Info("[DeadLetter]", slog.Int64("throttled", int64(i)))
	})

	actorSystem.ProcessRegistry.Add(dp, "deadletter")
	_ = actorSystem.EventStream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetterEvent); ok {

			// send back a response instead of timeout, including the target PID.
			if deadLetter.Sender != nil {
				actorSystem.Root.Send(deadLetter.Sender, &DeadLetterResponse{Target: deadLetter.PID})
			}

			// bail out if sender is set and deadletter request logging is false
			if !actorSystem.Config.DeadLetterRequestLogging && deadLetter.Sender != nil {
				return
			}

			if _, isIgnoreDeadLetter := deadLetter.Message.(IgnoreDeadLetterLogging); !isIgnoreDeadLetter {
				if shouldThrottle() == Open {
					actorSystem.Logger().Info("[DeadLetter]", slog.Any("pid", deadLetter.PID), slog.Any("message", deadLetter.Message), slog.Any("sender", deadLetter.Sender))
				}
			}
		}
	})

	// this subscriber may not be deactivated.
	// it ensures that Watch commands that reach a stopped actor gets a Terminated message back.
	// This can happen if one actor tries to Watch a PID, while another thread sends a Stop message.
	actorSystem.EventStream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetterEvent); ok {
			if watchMsg, ok := deadLetter.Message.(*Watch); ok {
				// we know that this is a local actor since we get it on our own event stream, thus the address is not terminated
				watchMsg.Watcher.sendSystemMessage(actorSystem, &Terminated{
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
	// unwrap the incoming envelope to access the actual message and sender
	_, msg, sender := UnwrapEnvelope(message)

	if dp.actorSystem.Config.MetricsEnabled {
		metricsSystem, ok := dp.actorSystem.Extensions.Get(extensionID).(*Metrics)
		if ok && metricsSystem.Enabled() {
			ctx := context.Background()
			if instruments := metricsSystem.metrics.Get(metrics.InternalActorMetrics); instruments != nil {
				labels := append(SystemLabels(dp.actorSystem),
					attribute.String("messagetype", MessageName(msg)),
				)

				instruments.DeadLetterCount.Add(ctx, 1, metric.WithAttributes(labels...))
			}
		}
	}

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
