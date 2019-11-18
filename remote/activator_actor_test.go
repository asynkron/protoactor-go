package remote

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ActivatorTestSuite struct {
	suite.Suite
}

func (suite *ActivatorTestSuite) SetupTest() {
	// Initialize package scoped variables

	// from activator_actor.go
	nameLookup = make(map[string]actor.Props)

	// from actor/process_registry.go
	actor.ProcessRegistry.RemoteHandlers = []actor.AddressResolver{}
}

func (suite *ActivatorTestSuite) TearDownTest() {
	// Reset package scoped variables so those tests run after this test suite won't be affected.

	// from activator_actor.go
	nameLookup = make(map[string]actor.Props)

	// from actor/process_registry.go
	actor.ProcessRegistry.RemoteHandlers = []actor.AddressResolver{}
}

func TestMockedRootContextSuite(t *testing.T) {
	suite.Run(t, new(ActivatorTestSuite))
}

func (suite *ActivatorTestSuite) TestSpawnFuture() {
	name := "name"
	kind := "kind"
	address := "192.0.2.0:1234"

	activator, activatorProcess := spawnMockProcess("activator")
	defer removeMockProcess(activator)

	var resolverCalled = false
	actor.ProcessRegistry.RemoteHandlers = []actor.AddressResolver{
		func(pid *actor.PID) (i actor.Process, b bool) {
			resolverCalled = true
			suite.Equal("activator", pid.Id)
			suite.Equal(address, pid.Address)
			return activatorProcess, true
		},
	}

	activatorProcess.On("SendUserMessage", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			var pid *actor.PID
			if suite.IsType(&actor.PID{}, args.Get(0)) {
				pid = args.Get(0).(*actor.PID)
				suite.Equal(activator.Id, pid.Id)
			}

			var envelope *actor.MessageEnvelope
			if suite.IsType(&actor.MessageEnvelope{}, args.Get(1)) {
				envelope = args.Get(1).(*actor.MessageEnvelope)
				message := envelope.Message
				if suite.IsType(&ActorPidRequest{}, message) {
					request := message.(*ActorPidRequest)
					suite.Equal(name, request.Name)
					suite.Equal(kind, request.Kind)
					suite.Equal(address, pid.Address)
				}
			}

			// Send the payload back to the sender Future
			rootContext.Send(envelope.Sender, &ActorPidResponse{
				Pid: &actor.PID{
					Address: pid.Address,
				},
				StatusCode: ResponseStatusCodeOK.ToInt32(),
			})
		}).
		Once()

	future := SpawnFuture(address, name, kind, 1*time.Second)

	suite.NotNil(future)
	result, err := future.Result()
	suite.Nil(err)
	if suite.IsType(&ActorPidResponse{}, result) {
		response := result.(*ActorPidResponse)
		suite.Equal(ResponseStatusCodeOK.ToInt32(), response.StatusCode)
		suite.Equal(address, response.Pid.GetAddress())
	}

	suite.True(resolverCalled, "AddressResolver should be called when message is sent over network.")
}

func (suite *ActivatorTestSuite) TestSpawn() {
	kind := "kind"
	address := "192.0.2.0:1234"

	activator, activatorProcess := spawnMockProcess("activator")
	defer removeMockProcess(activator)

	var resolverCalled = false
	actor.ProcessRegistry.RemoteHandlers = []actor.AddressResolver{
		func(pid *actor.PID) (i actor.Process, b bool) {
			resolverCalled = true
			suite.Equal("activator", pid.Id)
			suite.Equal(address, pid.Address)
			return activatorProcess, true
		},
	}

	activatorProcess.On("SendUserMessage", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			var pid *actor.PID
			if suite.IsType(&actor.PID{}, args.Get(0)) {
				pid = args.Get(0).(*actor.PID)
				suite.Equal(activator.Id, pid.Id)
			}

			var envelope *actor.MessageEnvelope
			if suite.IsType(&actor.MessageEnvelope{}, args.Get(1)) {
				envelope = args.Get(1).(*actor.MessageEnvelope)
				message := envelope.Message
				if suite.IsType(&ActorPidRequest{}, message) {
					request := message.(*ActorPidRequest)
					suite.Empty(request.Name, "No name is given by caller")
					suite.Equal(kind, request.Kind)
					suite.Equal(address, pid.Address)
				}
			}

			// Send the payload back to the sender Future
			rootContext.Send(envelope.Sender, &ActorPidResponse{
				Pid: &actor.PID{
					Address: pid.Address,
				},
				StatusCode: ResponseStatusCodeOK.ToInt32(),
			})
		}).
		Once()

	response, err := Spawn(address, kind, 100*time.Millisecond)

	suite.Nil(err)
	if suite.NotNil(response) {
		suite.Equal(ResponseStatusCodeOK.ToInt32(), response.StatusCode)
		suite.NotNil(response.Pid)
	}

	suite.True(resolverCalled, "AddressResolver should be called when message is sent over network.")
}

func (suite *ActivatorTestSuite) TestSpawnNamed() {
	tests := []struct {
		Name     string
		Kind     string
		Response interface{}
	}{
		{
			Name:     "name",
			Kind:     "kind",
			Response: nil, // Underlying Future should timeout
		},
		{
			Name: "name",
			Kind: "kind",
			Response: &ActorPidResponse{
				Pid:        &actor.PID{},
				StatusCode: ResponseStatusCodeOK.ToInt32(),
			},
		},
		{
			Name: "name",
			Kind: "kind",
			Response: &ActorPidResponse{
				Pid:        &actor.PID{},
				StatusCode: ResponseStatusCodePROCESSNAMEALREADYEXIST.ToInt32(),
			},
		},
		{
			Name: "name",
			Kind: "kind",
			Response: &ActorPidResponse{
				Pid:        nil,
				StatusCode: ResponseStatusCodeERROR.ToInt32(),
			},
		},
		{
			Name:     "name",
			Kind:     "kind",
			Response: struct{}{}, // Unknown structure is returned
		},
	}

	for i, tt := range tests {
		suite.Run(strconv.Itoa(i), func() {
			remoteAddress := "192.0.2.0:1234"
			activator, activatorProcess := spawnMockProcess("activator")
			defer removeMockProcess(activator)

			var resolverCalled = false
			actor.ProcessRegistry.RemoteHandlers = []actor.AddressResolver{
				func(pid *actor.PID) (i actor.Process, b bool) {
					resolverCalled = true
					suite.Equal("activator", pid.Id)
					suite.Equal(remoteAddress, pid.Address)
					return activatorProcess, true
				},
			}

			activatorProcess.On("SendUserMessage", mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					var pid *actor.PID
					if suite.IsType(&actor.PID{}, args.Get(0)) {
						pid = args.Get(0).(*actor.PID)
						suite.Equal(activator.Id, pid.Id)
					}

					var envelope *actor.MessageEnvelope
					if suite.IsType(&actor.MessageEnvelope{}, args.Get(1)) {
						envelope = args.Get(1).(*actor.MessageEnvelope)
						message := envelope.Message
						if suite.IsType(&ActorPidRequest{}, message) {
							request := message.(*ActorPidRequest)
							suite.Equal(tt.Name, request.Name)
							suite.Equal(tt.Kind, request.Kind)
						}
					}

					if tt.Response != nil {
						// Send the payload back to the sender Future
						rootContext.Send(envelope.Sender, tt.Response)
					}
				}).
				Once()

			response, err := SpawnNamed(remoteAddress, tt.Name, tt.Kind, 100*time.Millisecond)

			if tt.Response == nil {
				suite.Equal(actor.ErrTimeout, err)
				return
			}
			switch tt.Response.(type) {
			case *ActorPidResponse:
				suite.Equal(tt.Response, response)
			default:
				// When non *ActorPidResponse is returned from the underlying Future, that should be converted to an error.
				suite.Error(err)
			}

			suite.True(resolverCalled, "AddressResolver should be called when message is sent over network.")
		})
	}
}

func (suite *ActivatorTestSuite) TestRegister() {
	kind := "target"
	_, ok := nameLookup[kind]
	suite.False(ok, "Registering kind already exists: %s", kind)

	props := &actor.Props{}
	Register(kind, props)

	_, ok = nameLookup[kind]
	suite.True(ok, "Registered kind is not stored: %s", kind)
}

func (suite *ActivatorTestSuite) TestGetKnownKinds() {
	kinds := []string{"target1", "target2"}
	for _, kind := range kinds {
		nameLookup[kind] = actor.Props{}
	}

	knownKinds := GetKnownKinds()

	suite.ElementsMatch(kinds, knownKinds)
}

func (suite *ActivatorTestSuite) Test_activatorReceive_UnknownKind() {
	request := &ActorPidRequest{
		Name: "targetName",
		Kind: "targetKind",
	}
	context := &mockContext{}
	context.On("Message").Return(request).Once()
	context.On("Respond", mock.AnythingOfType("*remote.ActorPidResponse")).
		Run(func(args mock.Arguments) {
			suite.IsType(&ActorPidResponse{}, args.Get(0))
			response := args.Get(0).(*ActorPidResponse)
			suite.Equal(ResponseStatusCodeERROR.ToInt32(), response.StatusCode)
			suite.Nil(response.Pid)
		}).
		Once()

	activator := &activator{}
	suite.Panics(func() { activator.Receive(context) })

	context.AssertExpectations(suite.T())
}

func (suite *ActivatorTestSuite) Test_activatorReceive_Spawn() {
	name := "targetName"
	kind := "targetKind"

	uncontrollableErr := errors.New("uncontrollable")
	tests := []struct {
		TestName string
		HasKind  bool
		PidFunc  func(id string) *actor.PID
		Err      error
	}{
		{
			TestName: "initial call",
			HasKind:  true,
			PidFunc: func(id string) *actor.PID {
				return &actor.PID{
					Address: "nonhost",
					Id:      id,
				}
			},
			Err: nil,
		},
		{
			TestName: "subsequent call",
			HasKind:  true,
			PidFunc: func(id string) *actor.PID {
				return &actor.PID{
					Address: "nonhost",
					Id:      id,
				}
			},
			Err: actor.ErrNameExists,
		},
		{
			TestName: "activator error",
			HasKind:  true,
			PidFunc:  nil,
			Err: &ActivatorError{
				Code:       ResponseStatusCodeUNAVAILABLE.ToInt32(),
				DoNotPanic: false,
			},
		},
		{
			TestName: "unknown error",
			HasKind:  true,
			PidFunc:  nil,
			Err:      uncontrollableErr,
		},
		{
			TestName: "unknown kind",
			HasKind:  false,
			PidFunc:  nil,
			Err:      nil,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.TestName, func() {
			request := &ActorPidRequest{
				Name: name,
				Kind: kind,
			}

			nameLookup = make(map[string]actor.Props)
			if tt.HasKind {
				nameLookup[kind] = *actor.
					PropsFromFunc(func(c actor.Context) {}).
					WithSpawnFunc(func(id string, props *actor.Props, parentContext actor.SpawnerContext) (*actor.PID, error) {
						if tt.PidFunc == nil {
							return nil, tt.Err
						}
						return tt.PidFunc(id), tt.Err
					})
			}

			context := &mockContext{}
			context.On("Message").Return(request).Once() // A request for an actor
			context.
				On("Respond", mock.AnythingOfType("*remote.ActorPidResponse")).
				Run(func(args mock.Arguments) {
					suite.IsType(&ActorPidResponse{}, args.Get(0))
					response := args.Get(0).(*ActorPidResponse)
					if tt.Err != nil {
						// When an error is returned from spawn func, then the error should be properly translated.
						switch tt.Err.(type) {
						case *ActivatorError:
							suite.Equal(ResponseStatusCodeUNAVAILABLE.ToInt32(), response.StatusCode)
						default:
							if tt.Err == actor.ErrNameExists {
								suite.Equal(ResponseStatusCodePROCESSNAMEALREADYEXIST.ToInt32(), response.StatusCode)
							} else {
								suite.Equal(ResponseStatusCodeERROR.ToInt32(), response.StatusCode)
							}
						}
					}
					if !tt.HasKind {
						// When no corresponding kind is registered, then the response should contain an error.
						suite.Equal(ResponseStatusCodeERROR.ToInt32(), response.StatusCode)
					}
					if tt.PidFunc != nil {
						suite.NotNil(response.Pid)
						suite.Equal(actor.ProcessRegistry.Address, response.Pid.GetAddress())
						suite.Contains(response.Pid.GetId(), name)
						suite.Equal(fmt.Sprintf("Remote$%s", name), response.Pid.Id)
					}
				}).
				Once()

			e, ok := tt.Err.(*ActivatorError)
			if (ok && !e.DoNotPanic) ||
				tt.Err == uncontrollableErr ||
				!tt.HasKind {

				activator := &activator{}
				suite.Panics(func() {
					activator.Receive(context)
				})
				context.AssertExpectations(suite.T())
				return
			}

			activator := &activator{}
			activator.Receive(context)
			context.AssertExpectations(suite.T())
		})
	}

	suite.Run("ignorable message", func() {
		ignorables := []interface{}{
			&actor.Started{},
			&actor.Stopped{},
			&actor.Restart{},
			&actor.Failure{},
			&actor.Terminated{},
			&actor.Watch{},
			&actor.Unwatch{},
			&actor.PoisonPill{},
			&actor.Restarting{},
			&actor.Stopping{},
			&actor.Restarting{},
			struct{}{}, // Unknown type of message
		}

		for _, msg := range ignorables {
			suite.Run(fmt.Sprintf("%T", msg), func() {
				context := &mockContext{}
				context.On("Message").Return(msg).Once()

				activator := &activator{}
				activator.Receive(context)

				// Message is ignored and hence Respond is not called
				context.AssertNotCalled(suite.T(), "Respond")
				context.AssertExpectations(suite.T())
			})
		}
	})
}

func TestActivatorError_Error(t *testing.T) {
	var code int32 = 123
	err := &ActivatorError{Code: code}
	assert.Contains(t, err.Error(), fmt.Sprint(code), "Error message should contain stringified form of code")
}

func TestActivatorForAddress(t *testing.T) {
	address := "192.0.2.0"
	pid := ActivatorForAddress(address)
	assert.Equal(t, address, pid.Address)
}
