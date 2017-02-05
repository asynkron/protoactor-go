package gojcbmock

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type mockError struct {
	cause   error
	message string
}

func (e mockError) Error() string {
	return fmt.Sprintf("Mock Error: %s (caused by %s)", e.message, e.cause.Error())
}

func throwMockError(msg string, cause error) error {
	if cause == nil {
		cause = errors.New("No cause")
	}
	panic(mockError{message: msg, cause: cause})
}

const mockInitTimeout = 5 * time.Second

type BucketType int

const (
	BCouchbase BucketType = 0
	BMemcached            = iota
)

type BucketSpec struct {
	// Type of the bucket
	Type BucketType
	// Name of the bucket
	Name string
	// Password for the bucket (empty means no password)
	Password string
}

func (s BucketSpec) toString() string {
	specArr := make([]string, 3)
	specArr[0] = s.Name
	specArr[1] = s.Password
	if s.Type == BCouchbase {
		specArr[2] = "couchbase"
	} else {
		specArr[2] = "memcache"
	}
	return strings.Join(specArr, ":")
}

type Mock struct {
	// Executable object (for termination)
	cmd *exec.Cmd

	// Connection to the mock itself
	conn net.Conn

	// List of ports for the bucket
	EntryPort uint16

	// Internal reader-writer
	rw *bufio.ReadWriter
}

// Closes the mock and kills the underlying process
func (m *Mock) Close() {
	log.Printf("Closing mock %p\n", m)
	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Kill()
		m.cmd.Process.Wait()
	}
	if m.conn != nil {
		m.conn.Close()
	}
	m.EntryPort = 0
}

func (m *Mock) Control(c Command) (r Response) {
	reqbytes := c.Encode()
	reqbytes = append(reqbytes, '\n')
	if _, err := m.rw.Write(reqbytes); err != nil {
		throwMockError("Short write while sending command", err)
	}
	m.rw.Flush()
	log.Printf("Sent '%s'", reqbytes[:len(reqbytes)-1])

	resbytes, err := m.rw.ReadBytes('\n')
	log.Printf("Got '%s'", resbytes[:len(resbytes)-1])
	if err != nil {
		throwMockError("Short read while receiving response", err)
	}

	r = Response{Payload: make(map[string]interface{})}
	if err := json.Unmarshal(resbytes, &r.Payload); err != nil {
		throwMockError("Couldn't decode response JSON", err)
	}
	return r
}

func (m *Mock) MemcachedPorts() (out []uint16) {
	c := NewCommand(CGetMcPorts, nil)
	r := m.Control(c)
	if !r.Success() {
		throwMockError("Couldn't get memcached ports!", nil)
	}
	arr, ok := r.Payload["payload"].([]interface{})
	if !ok {
		throwMockError("Badly formatted port array", nil)
	}

	out = make([]uint16, 0)
	for _, v := range arr {
		tmpV, ok := v.(float64)
		if !ok {
			throwMockError(fmt.Sprintf("Expected numeric value. Got %T", v), nil)
		}
		out = append(out, uint16(tmpV))
	}
	return
}

func (m *Mock) buildSpecStrings(specs []BucketSpec) string {
	strSpecs := []string{}
	for _, spec := range specs {
		strSpecs = append(strSpecs, spec.toString())
	}
	return strings.Join(strSpecs, ",")
}

// Creates and runs a new mock instance
// The path is the path to the mock jar.
// nodes is the total number of cluster nodes (and thus the number of mock threads)
// replicas is the number of replica nodes (subset of the number of nodes) for each couchbase bucket.
// vbuckets is the number of vbuckets to use for each couchbase bucket
// specs should be a list of specifications of buckets to use..
func NewMock(path string, nodes uint, replicas uint, vbuckets uint, specs ...BucketSpec) (m *Mock, err error) {
	var lsn *net.TCPListener = nil
	chAccept := make(chan bool)
	m = &Mock{}

	defer func() {
		close(chAccept)
		if lsn != nil {
			lsn.Close()
		}
		exc := recover()

		if exc == nil {
			// No errors, everything is OK
			return
		}

		// Close mock on error, destroying resources
		m.Close()
		if mExc, ok := exc.(mockError); !ok {
			panic(mExc)
		} else {
			m = nil
			err = mExc
		}
	}()

	if lsn, err = net.ListenTCP("tcp", &net.TCPAddr{Port: 0}); err != nil {
		throwMockError("Couldn't set up listening socket", err)
	}
	_, ctlPort, _ := net.SplitHostPort(lsn.Addr().String())
	log.Printf("Listening for control connection at %s\n", ctlPort)

	go func() {
		var err error
		
		defer func() {
			chAccept <- false
		}()
		if m.conn, err = lsn.Accept(); err != nil {
			throwMockError("Couldn't accept incoming control connection from mock", err)
			return
		}
	}()

	if len(specs) == 0 {
		specs = []BucketSpec{BucketSpec{Name: "default", Type: BCouchbase}}
	}

	options := []string{
		"-jar", path, "--harakiri-monitor", "localhost:" + ctlPort, "--port", "0",
		"--replicas", strconv.Itoa(int(replicas)),
		"--vbuckets", strconv.Itoa(int(vbuckets)),
		"--nodes", strconv.Itoa(int(nodes)),
		"--buckets", m.buildSpecStrings(specs),
	}

	log.Printf("Invoking java %s", strings.Join(options, " "))
	m.cmd = exec.Command("java", options...)

	m.cmd.Stdout = os.Stdout
	m.cmd.Stderr = os.Stderr

	if err = m.cmd.Start(); err != nil {
		m.cmd = nil
		throwMockError("Couldn't start command", err)
	}

	select {
	case <-chAccept:
		break

	case <-time.After(mockInitTimeout):
		throwMockError("Timed out waiting for initalization", errors.New("timeout"))
	}

	m.rw = bufio.NewReadWriter(bufio.NewReader(m.conn), bufio.NewWriter(m.conn))

	// Read the port buffer, which is delimited by a NUL byte
	if portBytes, err := m.rw.ReadBytes(0); err != nil {
		throwMockError("Couldn't get port information", err)
	} else {
		portBytes = portBytes[:len(portBytes)-1]
		if entryPort, err := strconv.Atoi(string(portBytes)); err != nil {
			throwMockError("Incorrectly formatted port from mock", err)
		} else {
			m.EntryPort = uint16(entryPort)
		}
	}

	log.Printf("Mock HTTP port at %d\n", m.EntryPort)
	return
}

// Creates a default mock instance of 4 nodes
func NewMockDefault(path string, specs ...BucketSpec) (*Mock, error) {
	return NewMock(path, 4, 0, 32, specs...)
}
