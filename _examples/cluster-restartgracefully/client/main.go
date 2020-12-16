package main

import (
	"cluster-restartgracefully/shared"
	"flag"
	"fmt"
	"sync"
	"time"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/cluster/consul"
	"github.com/AsynkronIT/protoactor-go/cluster/etcd"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
)

var (
	system   = actor.NewActorSystem()
	plog     = log.New(log.DebugLevel, "[Example]")
	_cluster *cluster.Cluster
)

func main() {
	cluster.SetLogLevel(log.InfoLevel)
	var loops = flag.Int("loops", 10000, "request times.")
	var interval = flag.Duration("interval", 0, "request interval miliseconds per client.")
	var clients = flag.Int("clients", 1, "clients count.")
	var provider = flag.String("provider", "consul", "clients count.")
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
	case "etcd":
		cp, err = etcd.New()
	default:
		panic(fmt.Errorf("Invalid provider:%s", provider))
	}

	if err != nil {
		panic(err)
	}

	remoteCfg := remote.Configure("127.0.0.1", port)
	cfg := cluster.Configure("cluster-restartgracefully", cp, remoteCfg)
	_cluster = cluster.New(system, cfg)
	_cluster.StartClient()
}

func runClientsAll(clients int, loops int, interval time.Duration) {
	var wg sync.WaitGroup
	var now = time.Now()
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
		log.Int("clients", clients),
		log.Int("total", total),
		log.Int("req/s", total/costSecs),
		log.Duration("take", cost))
}

func runClient(grainId string, loops int, interval time.Duration) {
	callOpts := cluster.DefaultGrainCallOptions(_cluster).WithRetry(3).WithTimeout(6 * time.Second)
	now := time.Now()
	calcGrain := shared.GetCalculatorGrainClient(_cluster, grainId)
	resp, err := calcGrain.GetCurrent(&shared.Void{}, callOpts)
	if err != nil {
		_cluster.Shutdown(true)
		panic(err)
	}
	baseNumber := resp.Number
	plog.Info("requests",
		log.String("grainId", grainId),
		log.String("status", "start"))
	for i := 1; i <= loops; i++ {
		assert_calcAdd(grainId, 1, baseNumber+int64(i))
		time.Sleep(interval)
	}
	plog.Info("requests",
		log.String("grainId", grainId),
		log.String("status", "end"),
		log.Int("loops", loops),
		log.Duration("take", time.Since(now)))
}

func calcAdd(grainId string, addNumber int64) int64 {
	callOpts := cluster.DefaultGrainCallOptions(_cluster).WithRetry(3).WithTimeout(6 * time.Second)
	calcGrain := shared.GetCalculatorGrainClient(_cluster, grainId)
	resp, err := calcGrain.Add(&shared.NumberRequest{Number: addNumber}, callOpts)
	if err != nil {
		plog.Error("call grain failed", log.Error(err))
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
