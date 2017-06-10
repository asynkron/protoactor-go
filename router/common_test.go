package router

import (
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/stretchr/testify/mock"
)

var nilPID *actor.PID

func init() {
	// discard all logging in tests
	log.SetOutput(ioutil.Discard)
}

func spawnMockProcess(name string) (*actor.PID, *mockProcess) {
	p := &mockProcess{}
	pid, ok := actor.ProcessRegistry.Add(p, name)
	if !ok {
		panic(fmt.Errorf("did not spawn named process '%s'", name))
	}

	return pid, p
}

func removeMockProcess(pid *actor.PID) {
	actor.ProcessRegistry.Remove(pid)
}

type mockProcess struct {
	mock.Mock
}

func (m *mockProcess) SendUserMessage(pid *actor.PID, message interface{}) {
	msg, _ := actor.UnwrapEnvelope(message)
	m.Called(pid, msg)
}
func (m *mockProcess) SendSystemMessage(pid *actor.PID, message interface{}) {
	m.Called(pid, message)
}
func (m *mockProcess) Stop(pid *actor.PID) {
	m.Called(pid)
}

type mockContext struct {
	mock.Mock
}

func (m *mockContext) Watch(pid *actor.PID) {
	m.Called(pid)
}

func (m *mockContext) Unwatch(pid *actor.PID) {
	m.Called(pid)
}

func (m *mockContext) Message() interface{} {
	args := m.Called()
	return args.Get(0)
}

func (m *mockContext) SetReceiveTimeout(d time.Duration) {
	m.Called(d)
}
func (m *mockContext) ReceiveTimeout() time.Duration {
	args := m.Called()
	return args.Get(0).(time.Duration)
}

func (m *mockContext) Sender() *actor.PID {
	args := m.Called()
	return args.Get(0).(*actor.PID)
}

func (m *mockContext) MessageHeader() actor.ReadonlyMessageHeader {
	args := m.Called()
	return args.Get(0).(actor.ReadonlyMessageHeader)
}

func (m *mockContext) Tell(pid *actor.PID, message interface{}) {
	m.Called()
}

func (m *mockContext) Request(pid *actor.PID, message interface{}) {
	m.Called()
}

func (m *mockContext) SetBehavior(r actor.ActorFunc) {
	m.Called(r)
}

func (m *mockContext) PushBehavior(r actor.ActorFunc) {
	m.Called(r)
}

func (m *mockContext) PopBehavior() {
	m.Called()
}

func (m *mockContext) Self() *actor.PID {
	args := m.Called()
	return args.Get(0).(*actor.PID)
}

func (m *mockContext) Parent() *actor.PID {
	args := m.Called()
	return args.Get(0).(*actor.PID)
}

func (m *mockContext) Spawn(p *actor.Props) *actor.PID {
	args := m.Called(p)
	return args.Get(0).(*actor.PID)
}

func (m *mockContext) SpawnPrefix(p *actor.Props, prefix string) *actor.PID {
	args := m.Called(p, prefix)
	return args.Get(0).(*actor.PID)
}

func (m *mockContext) SpawnNamed(p *actor.Props, name string) (*actor.PID, error) {
	args := m.Called(p, name)
	return args.Get(0).(*actor.PID), args.Get(1).(error)
}

func (m *mockContext) Children() []*actor.PID {
	args := m.Called()
	return args.Get(0).([]*actor.PID)
}

func (m *mockContext) Stash() {
	m.Called()
}

func (m *mockContext) Respond(response interface{}) {
	m.Called(response)
}

func (m *mockContext) Actor() actor.Actor {
	args := m.Called()
	return args.Get(0).(actor.Actor)
}

func (m *mockContext) AwaitFuture(f *actor.Future, cont func(res interface{}, err error)) {
	m.Called(f, cont)
}

func (m *mockContext) RequestFuture(pid *actor.PID, message interface{}, timeout time.Duration) *actor.Future {
	args := m.Called()
	return args.Get(0).(*actor.Future)
}
