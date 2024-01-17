package main

import (
	"flag"
	"fmt"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"log/slog"
	"sync"
	"time"

	"cluster-restartgracefully/shared"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/remote"
)

var (
	system   = actor.NewActorSystem()
	plog     = slog.Default()
	_cluster *cluster.Cluster
)

func main() {

	loops := flag.Int("loops", 10000, "request times.")
	interval := flag.Duration("interval", 0, "request interval miliseconds per client.")
	clients := flag.Int("clients", 1, "clients count.")
	provider := flag.String("provider", "consul", "clients count.")
	flag.Parse()

	// start server
	startNode(0, *provider)
	for {
		runClientsAll(*clients, *loops, *interval)
		plog.Info("countinue? (y/n)")
		cmd, err := console.ReadLine()
		if err != nil {
			panic(err)
		}
		if cmd == "n" || cmd == "quit" {
			break
		}
	}
	plog.Info("shutdown ...")
	_cluster.Shutdown(true)
	plog.Info("shutdown OK")
}

func startNode(port int, provider string) {
	var cp cluster.ClusterProvider
	var err error
	switch provider {
	case "consul":
		ttl := consul.WithTTL(100 * time.Millisecond)
		refreshTTL := consul.WithRefreshTTL(100 * time.Millisecond)
		cp, err = consul.New(ttl, refreshTTL)
	// case "etcd":
	//	cp, err = etcd.New()
	default:
		panic(fmt.Errorf("invalid provider:%s", provider))
	}

	if err != nil {
		panic(err)
	}

	id := disthash.New()
	remoteCfg := remote.Configure("127.0.0.1", port)
	cfg := cluster.Configure("cluster-restartgracefully", cp, id, remoteCfg)
	_cluster = cluster.New(system, cfg)
	_cluster.StartClient()
}

func runClientsAll(clients int, loops int, interval time.Duration) {
	var wg sync.WaitGroup
	now := time.Now()
	for i := 0; i < clients; i++ {
		wg.Add(1)
		grainId := fmt.Sprintf("client-%d", i)
		go func() {
			runClient(grainId, loops, interval)
			wg.Done()
		}()
	}
	wg.Wait()
	cost := time.Since(now)
	total := clients * loops
	costSecs := int(cost / time.Second)
	if costSecs <= 0 {
		costSecs = 1
	}
	plog.Info("end all.",
		slog.Int("clients", clients),
		slog.Int("total", total),
		slog.Int("req/s", total/costSecs),
		slog.Duration("take", cost))
}

func runClient(grainId string, loops int, interval time.Duration) {
	now := time.Now()
	calcGrain := shared.GetCalculatorGrainClient(_cluster, grainId)
	resp, err := calcGrain.GetCurrent(&shared.Void{}, cluster.WithRetryCount(3), cluster.WithTimeout(6*time.Second))
	if err != nil {
		_cluster.Shutdown(true)
		panic(err)
	}
	baseNumber := resp.Number
	plog.Info("requests",
		slog.String("grainId", grainId),
		slog.String("status", "start"))
	for i := 1; i <= loops; i++ {
		assert_calcAdd(grainId, 1, baseNumber+int64(i))
		time.Sleep(interval)
	}
	plog.Info("requests",
		slog.String("grainId", grainId),
		slog.String("status", "end"),
		slog.Int("loops", loops),
		slog.Duration("take", time.Since(now)))
}

func calcAdd(grainId string, addNumber int64) int64 {
	calcGrain := shared.GetCalculatorGrainClient(_cluster, grainId)
	resp, err := calcGrain.Add(&shared.NumberRequest{Number: addNumber}, cluster.WithRetryCount(3), cluster.WithTimeout(6*time.Second))
	if err != nil {
		plog.Error("call grain failed", slog.Any("error", err))
	}
	return resp.Number
}

func assert_calcAdd(grainId string, inc, expectedNumber int64) {
	number := calcAdd(grainId, inc)
	if number != expectedNumber {
		err := fmt.Errorf("grainId:%s inc:%d number:%d (expected=%d)",
			grainId, inc, number, expectedNumber)
		panic(err)
	}
}
