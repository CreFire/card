package module

import (
	"fmt"
	"net/http"

	commonserver "backend/src/common/server"
)

type RobotModule struct {
	service *RobotService
	handler *Handler
}

func New(runtime *commonserver.Runtime) (*RobotModule, error) {
	if runtime == nil {
		return nil, fmt.Errorf("robot runtime is nil")
	}
	if runtime.Config == nil {
		return nil, fmt.Errorf("robot config is nil")
	}

	authClient := NewHTTPAuthClient(runtime)
	gateClient := NewHTTPGateClient(runtime)
	service := NewRobotService(authClient, gateClient)

	return &RobotModule{
		service: service,
		handler: NewHandler(service),
	}, nil
}

func (m *RobotModule) Init() error {
	return nil
}

func (m *RobotModule) RegisterHTTP(mux *http.ServeMux) {
	m.handler.RegisterHTTP(mux)
}
