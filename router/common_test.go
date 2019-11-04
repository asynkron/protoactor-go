package router

import (
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/otherview/protoactor-go/actor"
	"github.com/stretchr/testify/mock"
)

var nilPID *actor.PID

func init() {
	// discard all logging in tests
	log.SetOutput(ioutil.Discard)
}

// mockContext
type mockContext struct {
	mock.Mock
}

//
// Interface: Context
//

func (m *mockContext) Parent() *actor.PID {
	args := m.Called()
	return args.Get(0).(*actor.PID)
}

func (m *mockContext) Self() *actor.PID {
	args := m.Called()
	return args.Get(0).(*actor.PID)
}

func (m *mockContext) Sender() *actor.PID {
	args := m.Called()
	return args.Get(0).(*actor.PID)
}

func (m *mockContext) Actor() actor.Actor {
	args := m.Called()
	return args.Get(0).(actor.Actor)
}

func (m *mockContext) ReceiveTimeout() time.Duration {
	args := m.Called()
	return args.Get(0).(time.Duration)
}

func (m *mockContext) Children() []*actor.PID {
	args := m.Called()
	return args.Get(0).([]*actor.PID)
}

func (m *mockContext) Respond(response interface{}) {
	m.Called(response)
}

func (m *mockContext) Stash() {
	m.Called()
}

func (m *mockContext) Watch(pid *actor.PID) {
	m.Called(pid)
}

func (m *mockContext) Unwatch(pid *actor.PID) {
	m.Called(pid)
}

func (m *mockContext) SetReceiveTimeout(d time.Duration) {
	m.Called(d)
}

func (m *mockContext) CancelReceiveTimeout() {
	m.Called()
}

func (m *mockContext) Forward(pid *actor.PID) {
	m.Called()
}

func (m *mockContext) AwaitFuture(f *actor.Future, cont func(res interface{}, err error)) {
	m.Called(f, cont)
}

//
// Interface: SenderContext
//

func (m *mockContext) Message() interface{} {
	args := m.Called()
	return args.Get(0)
}

func (m *mockContext) MessageHeader() actor.ReadonlyMessageHeader {
	args := m.Called()
	return args.Get(0).(actor.ReadonlyMessageHeader)
}

func (m *mockContext) Send(pid *actor.PID, message interface{}) {
	m.Called()
	p, _ := actor.ProcessRegistry.Get(pid)
	p.SendUserMessage(pid, message)
}

func (m *mockContext) Request(pid *actor.PID, message interface{}) {
	args := m.Called()
	p, _ := actor.ProcessRegistry.Get(pid)
	env := &actor.MessageEnvelope{
		Header:  nil,
		Message: message,
		Sender:  args.Get(0).(*actor.PID),
	}
	p.SendUserMessage(pid, env)
}

func (m *mockContext) RequestWithCustomSender(pid *actor.PID, message interface{}, sender *actor.PID) {
	m.Called()
	p, _ := actor.ProcessRegistry.Get(pid)
	env := &actor.MessageEnvelope{
		Header:  nil,
		Message: message,
		Sender:  sender,
	}
	p.SendUserMessage(pid, env)
}

func (m *mockContext) RequestFuture(pid *actor.PID, message interface{}, timeout time.Duration) *actor.Future {
	args := m.Called()
	m.Called()
	p, _ := actor.ProcessRegistry.Get(pid)
	p.SendUserMessage(pid, message)
	return args.Get(0).(*actor.Future)
}

//
// Interface: ReceiverContext
//

func (m *mockContext) Receive(envelope *actor.MessageEnvelope) {
	m.Called(envelope)
}

//
// Interface: SpawnerContext
//

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

//
// Interface: StopperContext
//

func (m *mockContext) Stop(pid *actor.PID) {
	m.Called(pid)
}

func (m *mockContext) StopFuture(pid *actor.PID) *actor.Future {
	args := m.Called(pid)
	return args.Get(0).(*actor.Future)
}

func (m *mockContext) Poison(pid *actor.PID) {
	m.Called(pid)
}

func (m *mockContext) PoisonFuture(pid *actor.PID) *actor.Future {
	args := m.Called(pid)
	return args.Get(0).(*actor.Future)
}

// mockProcess
type mockProcess struct {
	mock.Mock
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

func (m *mockProcess) SendUserMessage(pid *actor.PID, message interface{}) {
	m.Called(pid, message)
}

func (m *mockProcess) SendSystemMessage(pid *actor.PID, message interface{}) {
	m.Called(pid, message)
}

func (m *mockProcess) Stop(pid *actor.PID) {
	m.Called(pid)
}
