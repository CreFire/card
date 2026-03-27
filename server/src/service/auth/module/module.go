package module

import (
	"context"
	"fmt"
	"net/http"
	"time"

	commonserver "backend/src/common/server"
)

type AuthModule struct {
	service *AuthService
	handler *Handler
}

func New(runtime *commonserver.Runtime) (*AuthModule, error) {
	if runtime == nil {
		return nil, fmt.Errorf("auth runtime is nil")
	}
	if runtime.Config == nil {
		return nil, fmt.Errorf("auth config is nil")
	}
	if runtime.Mongo == nil {
		return nil, fmt.Errorf("auth requires mongodb to be enabled")
	}
	if runtime.Redis == nil {
		return nil, fmt.Errorf("auth requires redis to be enabled")
	}

	accounts := NewAccountRepository(runtime.Mongo)
	sessions := NewSessionRepository(runtime.Redis)
	tickets := NewTicketRepository(runtime.Redis)
	connectTokens := NewConnectTokenRepository(runtime.Redis)
	service := NewAuthService(runtime.Config, accounts, sessions, tickets, connectTokens)

	return &AuthModule{
		service: service,
		handler: NewHandler(service),
	}, nil
}

func (m *AuthModule) Init() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return m.service.EnsureReady(ctx)
}

func (m *AuthModule) RegisterHTTP(mux *http.ServeMux) {
	m.handler.RegisterHTTP(mux)
}
