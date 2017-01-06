package consul_cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/AsynkronIT/protoactor-go/cluster"
)

type RegisterAgentService struct {
	ID                string   `json:"ID"`
	Name              string   `json:"Name"`
	Tags              []string `json:"Tags"`
	Address           string   `json:"Address"`
	Port              int      `json:"Port"`
	EnableTagOverride bool     `json:"EnableTagOverride"`
	Check             struct {
		DeregisterCriticalServiceAfter string `json:"DeregisterCriticalServiceAfter"`
		Script                         string `json:"Script"`
		HTTP                           string `json:"HTTP"`
		Interval                       string `json:"Interval"`
		TTL                            string `json:"TTL"`
	} `json:"Check"`
}

type ConsulProvider struct {
	shutdown bool
}

func New() *ConsulProvider {
	p := &ConsulProvider{}
	return p
}

func (p *ConsulProvider) RegisterMember(clusterName string, address string, port int, knownKinds []string) error {
	s := RegisterAgentService{
		ID:      fmt.Sprintf("%v_%v_%v", clusterName, address, port),
		Name:    clusterName,
		Tags:    knownKinds,
		Address: address,
		Port:    port,
	}

	b, err := json.Marshal(s)
	if err != nil {
		return err
	}

	url := "http://127.0.0.1"
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(b))
	req.Header.Set("X-Custom-Header", "myvalue")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Expected status 200, got: %v", resp.StatusCode)
	}
	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))

	return nil
}

func (p *ConsulProvider) Shutdown() error {
	p.shutdown = true
	return nil
}

func (p *ConsulProvider) GetStatusChanges() <-chan cluster.MemberStatus {
	c := make(chan cluster.MemberStatus)
	go func() {
		for !p.shutdown {

		}
	}()
	return c
}
