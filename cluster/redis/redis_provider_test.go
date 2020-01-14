package redis

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"

	"github.com/otherview/protoactor-go/cluster"
	"github.com/rafaeljusto/redigomock"
)

// TestRegisterMember tests a basic member registration and TTL update
func TestRegisterMember(t *testing.T) {

	clusterName := "mycluster"
	clusterAddress := "127.0.0.1"
	clusterPort := 8000
	kinds := []string{"a", "b"}

	node := NewNode(clusterName, clusterAddress, clusterPort, kinds)

	marshaled, err := json.Marshal(node)
	if err != nil {
		panic(err)
	}
	nodeString := string(marshaled)

	conn := redigomock.NewConn()
	conn.Command("SET", node.ID, nodeString, "EX", 10).ExpectStringSlice("OK")

	pool := &redis.Pool{
		Dial:    func() (redis.Conn, error) { return conn, nil },
		MaxIdle: 10,
	}

	p := NewWithPool(pool, 1*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	defer p.Shutdown()
	err = p.RegisterMember(clusterName, clusterAddress, clusterPort, kinds, nil, &cluster.NilMemberStatusValueSerializer{})
	if err != nil {
		log.Fatal(err)
	}

	time.Sleep(1 * time.Second)
	err = p.GetHealthStatus()
	if err != nil {
		log.Fatal(err)
	}
}

// TestErrorRegister tests an error registering a member
func TestErrorRegister(t *testing.T) {

	clusterName := "mycluster"
	clusterAddress := "127.0.0.1"
	clusterPort := 8000
	kinds := []string{"a", "b"}

	node := NewNode(clusterName, clusterAddress, clusterPort, kinds)

	marshaled, err := json.Marshal(node)
	if err != nil {
		panic(err)
	}
	nodeString := string(marshaled)

	derpMutex := new(sync.Mutex)

	conn := redigomock.NewConn()
	conn.Command("SET", node.ID, nodeString, "EX", 10).ExpectError(fmt.Errorf("TTL - error accessing Redis"))

	pool := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			derpMutex.Lock()
			defer derpMutex.Unlock()
			return conn, nil
		},
		MaxIdle: 2,
	}

	p := NewWithPool(pool, 1*time.Second)
	defer p.Shutdown()

	err = p.RegisterMember(clusterName, clusterAddress, clusterPort, kinds, nil, &cluster.NilMemberStatusValueSerializer{})
	if err != nil {
		log.Fatal(err)
	}

	time.Sleep(3 * time.Second)
	err = p.GetHealthStatus()
	if err == nil {
		log.Fatal(err)
	}

	derpMutex.Lock()
	conn.Clear()
	conn.Command("SET", node.ID, nodeString, "EX", 10).ExpectStringSlice("OK")
	derpMutex.Unlock()

	time.Sleep(2 * time.Second)
	err = p.GetHealthStatus()
	if err != nil {
		log.Fatal(err)
	}
}

//
//// TestMonitorMembers tests a basic member registration and Member Monitoring
//func TestMonitorMembers(t *testing.T) {
//	clusterName := "mycluster"
//	clusterAddress := "127.0.0.1"
//	clusterPort := 8000
//	kinds := []string{"a", "b"}
//
//	node := NewNode(clusterName, clusterAddress, clusterPort, kinds)
//
//	marshaled, err := json.Marshal(node)
//	if err != nil {
//		panic(err)
//	}
//	nodeString := string(marshaled)
//
//	conn := redigomock.NewConn()
//	conn.Command("SET", node.ID, nodeString, "EX", 10).ExpectStringSlice("OK")
//	conn.Command("GET", node.ID).Expect([]byte("{\"id\":\"mycluster/127.0.0.1:8000\",\"address\":\"127.0.0.1\",\"port\":8000,\"kinds\":[\"a\",\"b\"]}"))
//	conn.Command("SCAN", 0, "MATCH", fmt.Sprintf("%s*", clusterName)).Expect([]interface{}{"0", []interface{}{"mycluster/127.0.0.1:8000"}})
//
//	pool := &redis.Pool{
//		Dial:    func() (redis.Conn, error) { return conn, nil },
//		MaxIdle: 10,
//	}
//
//	p := NewWithPool(pool, 1*time.Second)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	defer p.Shutdown()
//	err = p.RegisterMember(clusterName, clusterAddress, clusterPort, kinds, nil, &cluster.NilMemberStatusValueSerializer{})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	p.MonitorMemberStatusChanges()
//	time.Sleep(2 * time.Second)
//	err = p.GetHealthStatus()
//	if err != nil {
//		log.Fatal(err)
//	}
//}
//
//// TestLocalConnection tests a basic member registration and Member Monitoring - locally
//func TestLocalConnection(t *testing.T) {
//	if testing.Short() {
//		return
//	}
//
//	clusterName := "mycluster"
//	clusterAddress := "127.0.0.1"
//	clusterPort := 8000
//	kinds := []string{"a", "b"}
//
//	p := New()
//
//	defer p.Shutdown()
//	err := p.RegisterMember(clusterName, clusterAddress, clusterPort, kinds, nil, &cluster.NilMemberStatusValueSerializer{})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	time.Sleep(2 * time.Second)
//	p.MonitorMemberStatusChanges()
//	time.Sleep(10 * time.Second)
//	err = p.GetHealthStatus()
//	if err != nil {
//		log.Fatal(err)
//	}
//}
