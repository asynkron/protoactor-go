package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cluster-restartgracefully/shared"

	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/remote"
)

var (
	system   = actor.NewActorSystem()
	_cluster *cluster.Cluster
	plog     = slog.Default()
)

func main() {
	provider := flag.String("provider", "consul", "clients count.")
	actorTTL := flag.Duration("ttl", 10*time.Second, "time to live of actor.")
	port := flag.Int("port", 0, "listen port.")

	flag.Parse()
	startNode(*port, *provider, *actorTTL)

	// waiting CTRL-C
	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, syscall.SIGINT)
	for sig := range sigCh {
		switch sig {
		case syscall.SIGINT:
			plog.Info("Shutdown...")
			_cluster.Shutdown(true)
			plog.Info("Shutdown ok")
			time.Sleep(time.Second)
			os.Exit(0)
		default:
			plog.Info("Skipping", slog.Any("sig", sig))
		}
	}
}

func startNode(port int, provider string, timeout time.Duration) {
	plog.Info("press 'CTRL-C' to shutdown server.")
	shared.CalculatorFactory(func() shared.Calculator {
		return &CalcGrain{}
	})

	var cp cluster.ClusterProvider
	var err error
	switch provider {
	case "consul":
		cp, err = consul.New()
	// case "etcd":
	//	cp, err = etcd.New()
	default:
		panic(fmt.Errorf("invalid provider:%s", provider))
	}

	id := disthash.New()

	if err != nil {
		panic(err)
	}

	remoteCfg := remote.Configure("127.0.0.1", port)
	cfg := cluster.Configure("cluster-restartgracefully", cp, id, remoteCfg, cluster.WithKinds(shared.GetCalculatorKind()))
	_cluster = cluster.New(system, cfg)
	_cluster.StartMember()
}
