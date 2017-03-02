package actor

import (
	"fmt"
	"time"

	"github.com/stretchr/testify/mock"
)

var nullProducer Producer = func() Actor { return nullReceive }
var nullReceive ActorFunc = func(Context) {}
var nilPID *PID

func matchPID(with *PID) interface{} {
	return mock.MatchedBy(func(v *PID) bool {
		return with.Address == v.Address && with.Id == v.Id
	})
}

func spawnMockProcess(name string) (*PID, *mockProcess) {
	p := &mockProcess{}
	pid, ok := ProcessRegistry.Add(p, name)
	if !ok {
		panic(fmt.Errorf("did not spawn named process '%s'", name))
	}

	return pid, p
}

func removeMockProcess(pid *PID) {
	ProcessRegistry.Remove(pid)
}

type mockProcess struct {
	mock.Mock
}

func (m *mockProcess) SendUserMessage(pid *PID, message interface{}, sender *PID) {
	m.Called(pid, message, sender)
}
func (m *mockProcess) SendSystemMessage(pid *PID, message interface{}) {
	m.Called(pid, message)
}
func (m *mockProcess) Stop(pid *PID) {
	m.Called(pid)
}

type mockContext struct {
	mock.Mock
}

func (m *mockContext) Watch(pid *PID) {
	m.Called(pid)
}

func (m *mockContext) Unwatch(pid *PID) {
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

func (m *mockContext) Sender() *PID {
	args := m.Called()
	return args.Get(0).(*PID)
}

func (m *mockContext) MessageHeader() ReadonlyMessageHeader {
	args := m.Called()
	return args.Get(0).(ReadonlyMessageHeader)
}

func (m *mockContext) Tell(pid *PID, message interface{}) {
	m.Called()
}

func (m *mockContext) Request(pid *PID, message interface{}) {
	m.Called()
}

func (m *mockContext) RequestFuture(pid *PID, message interface{}, timeout time.Duration) *Future {
	args := m.Called()
	return args.Get(0).(*Future)
}

func (m *mockContext) SetBehavior(r ActorFunc) {
	m.Called(r)
}

func (m *mockContext) PushBehavior(r ActorFunc) {
	m.Called(r)
}

func (m *mockContext) PopBehavior() {
	m.Called()
}

func (m *mockContext) Self() *PID {
	args := m.Called()
	return args.Get(0).(*PID)
}

func (m *mockContext) Parent() *PID {
	args := m.Called()
	return args.Get(0).(*PID)
}

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

func (m *mockContext) Children() []*PID {
	args := m.Called()
	return args.Get(0).([]*PID)
}

func (m *mockContext) Stash() {
	m.Called()
}

func (m *mockContext) Respond(response interface{}) {
	m.Called(response)
}

func (m *mockContext) Actor() Actor {
	args := m.Called()
	return args.Get(0).(Actor)
}

func (m *mockContext) AwaitFuture(f *Future, cont func(res interface{}, err error)) {
	m.Called(f, cont)
}
