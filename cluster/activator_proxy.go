package cluster

import (
	"log/slog"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

const (
	// proxyForwardTimeout is the default timeout for the proxy forwarding
	// an ActivationRequest to the local placement actor.
	proxyForwardTimeout = 10 * time.Second
)

// activatorProxy is a thin actor that receives remote ActivationRequest
// and ProxyActivationRequest messages and forwards them to the local
// placement actor. It provides a stable well-known name for cross-node
// communication.
type activatorProxy struct {
	placementPID *actor.PID
	lookup       IdentityLookup
}

// NewActivatorProxyProps returns actor Props for spawning the activator
// proxy. The proxy should be spawned as a named actor
// (e.g., "$proxy-activator") on each non-client member.
func NewActivatorProxyProps(placementPID *actor.PID, lookup IdentityLookup) *actor.Props {
	return actor.PropsFromProducer(func() actor.Actor {
		return &activatorProxy{
			placementPID: placementPID,
			lookup:       lookup,
		}
	})
}

func (a *activatorProxy) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started, *actor.Stopping, *actor.Stopped:
		// lifecycle messages — no-op
	case *ActivationRequest:
		a.forwardActivationRequest(ctx, msg)
	case *ProxyActivationRequest:
		a.handleProxyActivationRequest(ctx, msg)
	case *PeekRequest:
		a.forwardPeekRequest(ctx, msg)
	default:
		ctx.Logger().Debug("Activator proxy ignoring unknown message",
			slog.Any("type", msg))
	}
}

// forwardActivationRequest forwards an ActivationRequest to the local
// placement actor and responds with the result.
func (a *activatorProxy) forwardActivationRequest(ctx actor.Context, msg *ActivationRequest) {
	future := ctx.RequestFuture(a.placementPID, msg, proxyForwardTimeout)

	ctx.ReenterAfter(future, func(res any, err error) {
		if err != nil {
			ctx.Logger().Error("Proxy forward to placement actor failed",
				slog.String("identity", msg.ClusterIdentity.Identity),
				slog.Any("error", err))
			ctx.Respond(&ActivationResponse{Failed: true})
			return
		}

		// Forward the response as-is.
		ctx.Respond(res)
	})
}

// forwardPeekRequest forwards a PeekRequest to the local placement actor
// and responds with the result. This is a read-only operation.
func (a *activatorProxy) forwardPeekRequest(ctx actor.Context, msg *PeekRequest) {
	future := ctx.RequestFuture(a.placementPID, msg, proxyForwardTimeout)

	ctx.ReenterAfter(future, func(res any, err error) {
		if err != nil {
			ctx.Logger().Error("Proxy forward PeekRequest failed",
				slog.String("identity", msg.ClusterIdentity.Identity),
				slog.Any("error", err))
			ctx.Respond(&PeekResponse{Found: false})
			return
		}

		ctx.Respond(res)
	})
}

// handleProxyActivationRequest handles a ProxyActivationRequest. If
// ReplacedActivation is set, it calls RemovePid on the identity lookup
// to clean up the stale PID before forwarding the activation request.
func (a *activatorProxy) handleProxyActivationRequest(ctx actor.Context, msg *ProxyActivationRequest) {
	// If there's a stale PID to replace, remove it first.
	if msg.ReplacedActivation != nil && a.lookup != nil {
		a.lookup.RemovePid(msg.ClusterIdentity, msg.ReplacedActivation)
	}

	// Convert to ActivationRequest and forward.
	activationReq := &ActivationRequest{
		ClusterIdentity: msg.ClusterIdentity,
	}

	future := ctx.RequestFuture(a.placementPID, activationReq, proxyForwardTimeout)

	ctx.ReenterAfter(future, func(res any, err error) {
		if err != nil {
			ctx.Logger().Error("Proxy forward (ProxyActivationRequest) failed",
				slog.String("identity", msg.ClusterIdentity.Identity),
				slog.Any("error", err))
			ctx.Respond(&ActivationResponse{Failed: true})
			return
		}

		ctx.Respond(res)
	})
}
