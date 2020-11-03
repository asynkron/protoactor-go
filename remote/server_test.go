package remote

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"testing"
)

type ServerTestSuite struct {
	suite.Suite
	originalProcessRegistry *actor.ProcessRegistryValue
}

//func (suite *ServerTestSuite) SetupTest() {
//	// Initialize package scoped variables
//
//	// from server.go
//	s = nil
//	edpReader = nil
//
//	// from activator_actor.go
//	activatorPid = nil
//
//	// from endpoint_manager.go
//	endpointManager = nil
//}
//
//func (suite *ServerTestSuite) TearDownTest() {
//	if s != nil {
//		s.Stop() // Stop currently running gRPC server
//	}
//
//	// Reset package scoped variables so those tests run after this test suite won't be affected.
//
//	// from server.go
//	s = nil
//	edpReader = nil
//
//	// from activator_actor.go
//	activatorPid = nil
//
//	// from endpoint_manager.go
//	endpointManager = nil
//}

func TestStart(t *testing.T) {
	system := actor.NewActorSystem()
	config := BindTo("localhost", 0)
	remote := NewRemote(system, config)
	remote.Start()
	remote.Shutdown(true)
}

func TestConfig_WithAdvertisedHost(t *testing.T) {
	system := actor.NewActorSystem()
	config := BindTo("localhost", 0).WithAdvertisedHost("Banana")
	remote := NewRemote(system, config)
	remote.Start()
	assert.Equal(t, "Banana", system.ProcessRegistry.Address)
	remote.Shutdown(true)
}

//
//func (suite *ServerTestSuite) TestStart_AdvertisedAddress() {
//	// Find available port
//	lis, err := net.Listen("tcp", "127.0.0.1:0") // use :0 to choose available port
//	if err != nil {
//		panic(err)
//	}
//	//address := lis.Addr()
//	_ = lis.Close()
//
//	advertisedHost := "192.0.2.1:1234"
//	remote.Start()
//
//	suite.NotEmpty(system.ProcessRegistry.RemoteHandlers, "AddressResolver should be registered on server start")
//	suite.Equal(advertisedHost, system.ProcessRegistry.Address, "WithAdvertisedHost should have higher priority")
//	suite.NotNil(activatorPid, "Activator actor should be initialized on server start")
//	suite.NotNil(endpointManager, "EndpointManager should be initialized on server start")
//	suite.Equal(advertisedHost, remote.config.advertisedHost, "Passed configuration option should be used")
//	suite.NotNil(remote.edpReader, "EndpointReader should be initialized on server start")
//	suite.NotNil(remote.s, "gRPC server should be started on server start")
//}
//
//func (suite *ServerTestSuite) TestShutdown_Graceful() {
//	remote.edpReader = &endpointReader{}
//	suite.False(remote.edpReader.suspended, "EndpointReader should not be suspended at beginning")
//
//	endpointSupervisor, endpointSupervisorProcess := spawnMockProcess("EndpointSupervisor")
//	defer removeMockProcess(endpointSupervisor)
//	endpointSupervisorProcess.On("SendSystemMessage", mock.Anything, mock.Anything).
//		Run(func(args mock.Arguments) {
//			if suite.IsType(&actor.PID{}, args.Get(0)) {
//				pid := args.Get(0).(*actor.PID)
//				suite.Equal(endpointSupervisor, pid)
//			}
//			if suite.IsType(&actor.Watch{}, args.Get(1)) {
//				watch := args.Get(1).(*actor.Watch)
//				system.Root.Send(watch.Watcher, &actor.Terminated{
//					Who:               endpointSupervisor,
//					AddressTerminated: false,
//				})
//			}
//		}).
//		Once()
//	endpointSupervisorProcess.On("Stop", endpointSupervisor).Once()
//
//	endpointManager = &endpointManagerValue{
//		connections:        &sync.Map{},
//		remote:             remote,
//		endpointSupervisor: endpointSupervisor,
//		endpointSub:        system.EventStream.Subscribe(func(evt interface{}) {}),
//	}
//
//	var activatorProcess *mockProcess
//	activatorPid, activatorProcess = spawnMockProcess("activator")
//	defer removeMockProcess(activatorPid)
//	activatorProcess.On("SendSystemMessage", mock.Anything, mock.Anything).
//		Run(func(args mock.Arguments) {
//			if suite.IsType(&actor.PID{}, args.Get(0)) {
//				pid := args.Get(0).(*actor.PID)
//				suite.Equal(activatorPid, pid)
//			}
//			if suite.IsType(&actor.Watch{}, args.Get(1)) {
//				watch := args.Get(1).(*actor.Watch)
//				system.Root.Send(watch.Watcher, &actor.Terminated{
//					Who:               activatorPid,
//					AddressTerminated: false,
//				})
//			}
//		}).
//		Once()
//	activatorProcess.On("Stop", activatorPid).Once()
//
//	lis, err := net.Listen("tcp", "127.0.0.1:0") // use :0 to choose available port
//	if err != nil {
//		panic(err)
//	}
//	defer lis.Close()
//
//	grpcStopped := make(chan struct{}, 1)
//	remote.s = grpc.NewServer()
//	go func() {
//		remote.s.Serve(lis)
//		grpcStopped <- struct{}{}
//	}()
//
//	remote.Shutdown(true)
//
//	suite.Nil(endpointManager.endpointSub, "Subscription should reset on shutdown")
//	suite.Nil(endpointManager.connections, "Connections should reset on shutdown")
//
//	select {
//	case <-time.NewTimer(15 * time.Second).C:
//		suite.FailNow("gRPC server did not stop")
//	case <-grpcStopped:
//		// O.K.
//	}
//
//	endpointSupervisorProcess.AssertExpectations(suite.T())
//}
//
//func (suite *ServerTestSuite) TestShutdown() {
//	remote.edpReader = &endpointReader{}
//	suite.False(remote.edpReader.suspended, "EndpointReader should not be suspended at beginning")
//
//	endpointSupervisor, endpointSupervisorProcess := spawnMockProcess("EndpointSupervisor")
//	defer removeMockProcess(endpointSupervisor)
//
//	var activatorProcess *mockProcess
//	activatorPid, activatorProcess = spawnMockProcess("activator")
//	defer removeMockProcess(activatorPid)
//
//	endpointManager = &endpointManagerValue{
//		connections:        &sync.Map{},
//		remote:             nil,
//		endpointSupervisor: endpointSupervisor,
//		endpointSub:        system.EventStream.Subscribe(func(evt interface{}) {}),
//	}
//
//	lis, err := net.Listen("tcp", "127.0.0.1:0") // use :0 to choose available port
//	if err != nil {
//		panic(err)
//	}
//	defer lis.Close()
//
//	grpcStopped := make(chan struct{}, 1)
//	remote.s = grpc.NewServer()
//	go func() {
//		remote.s.Serve(lis)
//		grpcStopped <- struct{}{}
//	}()
//
//	remote.Shutdown(false)
//
//	suite.NotNil(endpointManager.endpointSub, "Subscription should not reset on non-graceful shutdown")
//	suite.NotNil(endpointManager.connections, "Connections should not reset on non-graceful shutdown")
//
//	select {
//	case <-time.NewTimer(1 * time.Second).C:
//		suite.FailNow("gRPC server did not stop")
//	case <-grpcStopped:
//		// O.K.
//	}
//
//	activatorProcess.AssertNotCalled(suite.T(), "SendSystemMessage", mock.Anything, mock.Anything)
//	activatorProcess.AssertExpectations(suite.T())
//	endpointSupervisorProcess.AssertNotCalled(suite.T(), "SendSystemMessage", mock.Anything, mock.Anything)
//	endpointSupervisorProcess.AssertExpectations(suite.T())
//}
