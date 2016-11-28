package messages

import "github.com/AsynkronIT/gam/actor"

type chatServer struct {
	clients []*actor.PID
}

func (server *chatServer) Say(r *SayRequest) (*Unit, error) {
	server.broadcast(r)
	return &Unit{}, nil
}

func (server *chatServer) Nick(r *NickRequest) (*Unit, error) {
	server.broadcast(r)
	return &Unit{}, nil
}

func (server *chatServer) Connect(r *ConnectRequest) (*ConnectResponse, error) {
	server.clients = append(server.clients, r.ClientStreamPID)
	return &ConnectResponse{
		Message: "Welcome to GAM Cluster Chat",
	}, nil
}

func (server *chatServer) broadcast(message interface{}) {
	for _, pid := range server.clients {
		pid.Tell(message)
	}
}

func init() {
	ChatServerFactory(func() ChatServer {
		return &chatServer{}
	})
}
