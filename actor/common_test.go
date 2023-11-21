package actor

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/asynkron/protoactor-go/ctxext"

	"github.com/stretchr/testify/mock"
)

var (
	nullProducer Producer    = func() Actor { return nullReceive }
	nullReceive  ReceiveFunc = func(Context) {}
	system                   = NewActorSystem()
	rootContext              = system.Root
)

// mockContext
type mockContext struct {
	mock.Mock
}

func (m *mockContext) Logger() *slog.Logger {
	return slog.Default()
}

//
// Interface: Context
//

func (m *mockContext) ActorSystem() *ActorSystem {
	args := m.Called()
	return args.Get(0).(*ActorSystem)
}

func (m *mockContext) Get(id ctxext.ContextExtensionID) ctxext.ContextExtension {
	args := m.Called(id)
	return args.Get(0).(ctxext.ContextExtension)
}

func (m *mockContext) Set(ext ctxext.ContextExtension) {
	m.Called(ext)
}

func (m *mockContext) Parent() *PID {
	args := m.Called()
	return args.Get(0).(*PID)
}

func (m *mockContext) Self() *PID {
	args := m.Called()
	return args.Get(0).(*PID)
}

func (m *mockContext) Sender() *PID {
	args := m.Called()
	return args.Get(0).(*PID)
}

func (m *mockContext) Actor() Actor {
	args := m.Called()
	return args.Get(0).(Actor)
}

func (m *mockContext) ReceiveTimeout() time.Duration {
	args := m.Called()
	return args.Get(0).(time.Duration)
}

func (m *mockContext) Children() []*PID {
	args := m.Called()
	return args.Get(0).([]*PID)
}

func (m *mockContext) Respond(response interface{}) {
	m.Called(response)
}

func (m *mockContext) Stash() {
	m.Called()
}

func (m *mockContext) Watch(pid *PID) {
	m.Called(pid)
}

func (m *mockContext) Unwatch(pid *PID) {
	m.Called(pid)
}

func (m *mockContext) SetReceiveTimeout(d time.Duration) {
	m.Called(d)
}

func (m *mockContext) CancelReceiveTimeout() {
	m.Called()
}

func (m *mockContext) Forward(_ *PID) {
	m.Called()
}

func (m *mockContext) ReenterAfter(f *Future, cont func(res interface{}, err error)) {
	m.Called(f, cont)
}

//
// Interface: SenderContext
//

func (m *mockContext) Message() interface{} {
	args := m.Called()

	return args.Get(0)
}

func (m *mockContext) MessageHeader() ReadonlyMessageHeader {
	args := m.Called()

	return args.Get(0).(ReadonlyMessageHeader)
}

func (m *mockContext) Send(_ *PID, _ interface{}) {
	m.Called()
}

func (m *mockContext) Request(pid *PID, message interface{}) {
	args := m.Called()

	p, _ := system.ProcessRegistry.Get(pid)
	env := &MessageEnvelope{
		Header:  nil,
		Message: message,
		Sender:  args.Get(0).(*PID),
	}
	p.SendUserMessage(pid, env)
}

func (m *mockContext) RequestWithCustomSender(pid *PID, message interface{}, sender *PID) {
	m.Called()

	p, _ := system.ProcessRegistry.Get(pid)
	env := &MessageEnvelope{
		Header:  nil,
		Message: message,
		Sender:  sender,
	}
	p.SendUserMessage(pid, env)
}

func (m *mockContext) RequestFuture(_ *PID, _ interface{}, _ time.Duration) *Future {
	args := m.Called()

	return args.Get(0).(*Future)
}

//
// Interface: ReceiverContext
//

func (m *mockContext) Receive(envelope *MessageEnvelope) {
	m.Called(envelope)
}

//
// Interface: SpawnerContext
//

func (m *mockContext) Spawn(p *Props) *PID {
	args := m.Called(p)
	return args.Get(0).(*PID)
}

func (m *mockContext) SpawnPrefix(p *Props, prefix string) *PID {
	args := m.Called(p, prefix)

	return args.Get(0).(*PID)
}

func (m *mockContext) SpawnNamed(p *Props, name string) (*PID, error) {
	args := m.Called(p, name)

	return args.Get(0).(*PID), args.Get(1).(error)
}

// mockProcess
type mockProcess struct {
	mock.Mock
}

func spawnMockProcess(name string) (*PID, *mockProcess) {
	p := &mockProcess{}

	pid, ok := system.ProcessRegistry.Add(p, name)
	if !ok {
		panic(fmt.Errorf("did not spawn named process '%vids'", name))
	}

	return pid, p
}

func removeMockProcess(pid *PID) {
	system.ProcessRegistry.Remove(pid)
}

func (m *mockProcess) SendUserMessage(pid *PID, message interface{}) {
	m.Called(pid, message)
}

func (m *mockProcess) SendSystemMessage(pid *PID, message interface{}) {
	m.Called(pid, message)
}

func (m *mockProcess) Stop(pid *PID) {
	m.Called(pid)
}
