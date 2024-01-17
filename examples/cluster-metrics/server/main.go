package main

import (
	"flag"
	"log"
	"time"

	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"

	"cluster-metrics/shared"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/remote"
)

func Logger(next actor.ReceiverFunc) actor.ReceiverFunc {
	fn := func(context actor.ReceiverContext, env *actor.MessageEnvelope) {
		switch env.Message.(type) {
		case *actor.Started:
			log.Printf("actor started " + context.Self().String())
		case *actor.Stopped:
			log.Printf("actor stopped " + context.Self().String())
		}
		next(context, env)
	}

	return fn
}

func newHelloActor() actor.Actor {
	return &shared.HelloActor{
		Timeout: 20 * time.Second,
	}
}

// a Go struct implementing the Hello interface
type HelloGrain struct{}

func (h *HelloGrain) Init(ctx cluster.GrainContext) {
	log.Printf("new grain id=%s", ctx.Identity)
}

func (h *HelloGrain) Terminate(ctx cluster.GrainContext) {
	log.Printf("delete grain id=%s", ctx.Identity())
}

func (*HelloGrain) ReceiveDefault(ctx cluster.GrainContext) {
	msg := ctx.Message()
	log.Printf("Unknown message %v", msg)
}

func (h *HelloGrain) SayHello(r *shared.HelloRequest, ctx cluster.GrainContext) (*shared.HelloResponse, error) {
	return &shared.HelloResponse{Message: "hello " + r.Name + " from " + ctx.Identity()}, nil
}

func (*HelloGrain) Add(r *shared.AddRequest, ctx cluster.GrainContext) (*shared.AddResponse, error) {
	return &shared.AddResponse{Result: r.A + r.B}, nil
}

func (*HelloGrain) VoidFunc(r *shared.AddRequest, ctx cluster.GrainContext) (*shared.Unit, error) {
	return &shared.Unit{}, nil
}

func main() {
	port := flag.Int("port", 0, "")
	flag.Parse()
	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", *port)
	helloKind := shared.NewHelloKind(func() shared.Hello { return &HelloGrain{} }, 0, actor.WithReceiverMiddleware(Logger))

	provider, _ := consul.New()
	lookup := disthash.New()

	clusterConfig := cluster.Configure("my-cluster", provider, lookup, remoteConfig, cluster.WithKinds(helloKind))
	c := cluster.New(system, clusterConfig)
	c.StartMember()

	// this node knows about Hello kind
	hello := shared.GetHelloGrainClient(c, "MyGrain")
	msg := &shared.HelloRequest{Name: "Roger"}
	res, err := hello.SayHello(msg)
	if err != nil {
		log.Fatalf("failed to call SayHello, err:%v", err)
	}
	log.Printf("Message from grain: %v", res.Message)
	_, _ = console.ReadLine()
	c.Shutdown(true)
}
