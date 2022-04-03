package main

import (
	"flag"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"log"
	"time"

	"cluster-metrics/shared"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	logmod "github.com/asynkron/protoactor-go/log"
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
type HelloGrain struct {
	cluster.Grain
}

func (h *HelloGrain) Init(ci *cluster.ClusterIdentity, cl *cluster.Cluster) {
	h.Grain.Init(ci, cl)
	log.Printf("new grain id=%s", ci.Identity)
}

func (h *HelloGrain) Terminate() {
	log.Printf("delete grain id=%s", h.Grain.Identity())
}

func (*HelloGrain) ReceiveDefault(ctx actor.Context) {
	msg := ctx.Message()
	log.Printf("Unknown message %v", msg)
}

func (h *HelloGrain) SayHello(r *shared.HelloRequest, ctx cluster.GrainContext) (*shared.HelloResponse, error) {
	return &shared.HelloResponse{Message: "hello " + r.Name + " from " + h.Identity()}, nil
}

func (*HelloGrain) Add(r *shared.AddRequest, ctx cluster.GrainContext) (*shared.AddResponse, error) {
	return &shared.AddResponse{Result: r.A + r.B}, nil
}

func (*HelloGrain) VoidFunc(r *shared.AddRequest, ctx cluster.GrainContext) (*shared.Unit, error) {
	return &shared.Unit{}, nil
}

func main() {
	shared.HelloFactory(func() shared.Hello { return &HelloGrain{} })

	port := flag.Int("port", 0, "")
	flag.Parse()
	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", *port)
	helloKind := shared.GetHelloKind(actor.WithReceiverMiddleware(Logger))
	cluster.SetLogLevel(logmod.InfoLevel)

	provider, _ := consul.New()
	lookup := disthash.New()

	clusterConfig := cluster.Configure("my-cluster", provider, lookup, remoteConfig, cluster.WithKind(helloKind))
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
