package module

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type GateService struct {
	authClient AuthClient
	gamers     *GamerManager
}

func NewGateService(authClient AuthClient, gamers *GamerManager) *GateService {
	return &GateService{
		authClient: authClient,
		gamers:     gamers,
	}
}

func (s *GateService) OpenConnection(_ context.Context, input OpenConnectionInput) (*Gamer, error) {
	return s.gamers.Open(strings.TrimSpace(input.RemoteAddr), nil)
}

func (s *GateService) OpenSocket(_ context.Context, remoteAddr string, closer io.Closer) (*Gamer, error) {
	return s.gamers.Open(strings.TrimSpace(remoteAddr), closer)
}

func (s *GateService) LoginByToken(ctx context.Context, input AttachTokenInput) (*Gamer, error) {
	connID := strings.TrimSpace(input.ConnID)
	token := strings.TrimSpace(input.Token)
	if connID == "" {
		return nil, fmt.Errorf("%w: conn_id is required", ErrInvalidArgument)
	}
	if token == "" {
		return nil, fmt.Errorf("%w: token is required", ErrInvalidArgument)
	}

	session, err := s.authClient.ConsumeConnectToken(ctx, token)
	if err != nil {
		return nil, err
	}
	return s.gamers.BindSession(connID, session)
}

func (s *GateService) LoginByTicket(ctx context.Context, input AttachTicketInput) (*Gamer, error) {
	connID := strings.TrimSpace(input.ConnID)
	ticket := strings.TrimSpace(input.Ticket)
	if connID == "" {
		return nil, fmt.Errorf("%w: conn_id is required", ErrInvalidArgument)
	}
	if ticket == "" {
		return nil, fmt.Errorf("%w: ticket is required", ErrInvalidArgument)
	}

	session, err := s.authClient.ConsumeLoginTicket(ctx, ticket)
	if err != nil {
		return nil, err
	}
	return s.gamers.BindSession(connID, session)
}

func (s *GateService) LoginBySession(ctx context.Context, input AttachSessionInput) (*Gamer, error) {
	connID := strings.TrimSpace(input.ConnID)
	sessionID := strings.TrimSpace(input.SessionID)
	if connID == "" {
		return nil, fmt.Errorf("%w: conn_id is required", ErrInvalidArgument)
	}
	if sessionID == "" {
		return nil, fmt.Errorf("%w: session_id is required", ErrInvalidArgument)
	}

	session, err := s.authClient.ValidateSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return s.gamers.BindSession(connID, session)
}

func (s *GateService) Logout(_ context.Context, input DisconnectInput) error {
	connID := strings.TrimSpace(input.ConnID)
	if connID == "" {
		return fmt.Errorf("%w: conn_id is required", ErrInvalidArgument)
	}
	return s.gamers.Disconnect(connID)
}

func (s *GateService) Disconnect(ctx context.Context, input DisconnectInput) error {
	return s.Logout(ctx, input)
}

func (s *GateService) GetGamer(_ context.Context, connID string) (*Gamer, error) {
	connID = strings.TrimSpace(connID)
	if connID == "" {
		return nil, fmt.Errorf("%w: conn_id is required", ErrInvalidArgument)
	}
	return s.gamers.Get(connID)
}

func (s *GateService) ListGamers(_ context.Context) []*Gamer {
	return s.gamers.List()
}

func (s *GateService) Stats(_ context.Context) GateStats {
	return s.gamers.Stats()
}

func (s *GateService) CloseAll() {
	s.gamers.CloseAll()
}
