package automanaged

import (
	"log"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo"

	"github.com/otherview/protoactor-go/cluster"
)

var (
	mockData = new(sync.Mutex)
)

// TestRegisterMember tests a basic member registration and TTL update
func TestRegisterMember(t *testing.T) {

	clusterName := "mycluster"
	clusterAddress := "127.0.0.1"
	clusterPort := 6333
	kinds := []string{"a", "b"}

	p := New()
	defer p.Shutdown()
	err := p.RegisterMember(clusterName, clusterAddress, clusterPort, kinds, nil, &cluster.NilMemberStatusValueSerializer{})
	if err != nil {
		log.Fatal(err)
	}

	time.Sleep(2 * time.Second)
	err = p.GetHealthStatus()
	if err != nil {
		log.Fatal(err)
	}

	p.MonitorMemberStatusChanges()
	time.Sleep(5 * time.Second)
	err = p.GetHealthStatus()
	if err != nil {
		log.Fatal(err)
	}
}

// TestErrorRegister tests an error registering a member
func TestErrorRegister(t *testing.T) {

	clusterName := "mycluster"
	clusterAddress := "127.0.0.1"
	clusterPort := 6333
	kinds := []string{"a", "b"}

	callMock := CallMocker{}

	e := echo.New()
	e.HideBanner = true
	defer e.Close()

	callMock.setMockData(http.StatusBadRequest, nil)
	e.GET("/_health", func(context echo.Context) error {
		return context.JSON(callMock.getMockData())
	})

	p := NewWithConfig(2*time.Second, e, clusterPort, true, "localhost:6333")
	defer p.Shutdown()

	err := p.RegisterMember(clusterName, clusterAddress, clusterPort, kinds, nil, &cluster.NilMemberStatusValueSerializer{})
	if err != nil {
		log.Fatal(err)
	}

	time.Sleep(2 * time.Second)
	err = p.GetHealthStatus()
	if err != nil {
		log.Fatal(err)
	}

	p.MonitorMemberStatusChanges()
	time.Sleep(2 * time.Second)
	err = p.GetHealthStatus()
	if err == nil {
		log.Fatal(err)
	}

	node := NewNode(clusterName, clusterAddress, clusterPort, kinds)
	callMock.setMockData(http.StatusOK, node)

	time.Sleep(2 * time.Second)
	err = p.GetHealthStatus()
	if err != nil {
		log.Fatal(err)
	}

	e.Close()
	time.Sleep(2 * time.Second)
	err = p.GetHealthStatus()
	if err == nil {
		log.Fatal(err)
	}

}

type CallMocker struct {
	httpCode int
	data     interface{}
}

func (c *CallMocker) getMockData() (code int, i interface{}) {
	mockData.Lock()
	defer mockData.Unlock()

	return c.httpCode, c.data
}

func (c *CallMocker) setMockData(code int, i interface{}) {
	mockData.Lock()
	defer mockData.Unlock()

	c.httpCode = code
	c.data = i
}
