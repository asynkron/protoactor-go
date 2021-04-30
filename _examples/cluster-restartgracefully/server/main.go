package main

import (
	"cluster-restartgracefully/shared"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/cluster/consul"
	"github.com/AsynkronIT/protoactor-go/cluster/etcd"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
)

var (
	plog     = log.New(log.DebugLevel, "[Example]")
	system   = actor.NewActorSystem()
	_cluster *cluster.Cluster
)

func main() {
	var provider = flag.String("provider", "consul", "clients count.")
	var actorTTL = flag.Duration("ttl", 10*time.Second, "time to live of actor.")
	var port = flag.Int("port", 0, "listen port.")

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
			plog.Info("Skipping", log.Object("sig", sig))
		}
	}
}

func startNode(port int, provider string, timeout time.Duration) {
	plog.Info("press 'CTRL-C' to shutdown server.")
	shared.CalculatorFactory(func() shared.Calculator {
		return &shared.CalcGrain{}
	})

	var cp cluster.ClusterProvider
	var err error
	switch provider {
	case "consul":
		cp, err = consul.New()
	case "etcd":
		cp, err = etcd.New()
	default:
		panic(fmt.Errorf("Invalid provider:%s", provider))
	}

	if err != nil {
		panic(err)
	}

	kind := cluster.NewKind("Calculator", actor.PropsFromProducer(func() actor.Actor {
		return &shared.CalculatorActor{
			Timeout: timeout,
		}
	}))
	remoteCfg := remote.Configure("127.0.0.1", port)
	cfg := cluster.Configure("cluster-restartgracefully", cp, remoteCfg, kind)
	_cluster = cluster.New(system, cfg)
	_cluster.Start()
}
