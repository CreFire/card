package module

import (
	"fmt"
	"net/http"

	commonserver "backend/src/common/server"
)

type GateModule struct {
	service    *GateService
	handler    *Handler
	wsServer   *WebSocketServer
	tcpServer  *TCPServer
	grpcServer *GRPCServer
}

func New(runtime *commonserver.Runtime) (*GateModule, error) {
	if runtime == nil {
		return nil, fmt.Errorf("gate runtime is nil")
	}
	if runtime.Config == nil {
		return nil, fmt.Errorf("gate config is nil")
	}

	gamers := NewGamerManager()
	authClient := NewHTTPAuthClient(runtime)
	service := NewGateService(authClient, gamers)

	return &GateModule{
		service:    service,
		handler:    NewHandler(service),
		wsServer:   NewWebSocketServer(service),
		tcpServer:  NewTCPServer(runtime.Config.TCPListenAddress(), service),
		grpcServer: NewGRPCServer(runtime.Config.GRPCListenAddress(), service),
	}, nil
}

func (m *GateModule) Init() error {
	return nil
}

func (m *GateModule) Start() error {
	if err := m.tcpServer.Start(); err != nil {
		return err
	}
	if err := m.grpcServer.Start(); err != nil {
		_ = m.tcpServer.Stop()
		return err
	}
	return nil
}

func (m *GateModule) Stop() error {
	if m.grpcServer != nil {
		m.grpcServer.Stop()
	}
	if m.tcpServer != nil {
		return m.tcpServer.Stop()
	}
	return nil
}

func (m *GateModule) RegisterHTTP(mux *http.ServeMux) {
	m.handler.RegisterHTTP(mux)
	if m.wsServer != nil {
		m.wsServer.RegisterHTTP(mux)
	}
}
