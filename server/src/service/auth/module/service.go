package module

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"backend/src/common/configdoc"
)

const (
	defaultSessionTTL     = 24 * time.Hour
	defaultLoginTicketTTL = 60 * time.Second
	defaultConnectTTL     = 5 * time.Minute
)

type AuthService struct {
	cfg           *configdoc.ConfigBase
	accounts      *AccountRepository
	sessions      *SessionRepository
	tickets       *TicketRepository
	connectTokens *ConnectTokenRepository
}

func NewAuthService(
	cfg *configdoc.ConfigBase,
	accounts *AccountRepository,
	sessions *SessionRepository,
	tickets *TicketRepository,
	connectTokens *ConnectTokenRepository,
) *AuthService {
	return &AuthService{
		cfg:           cfg,
		accounts:      accounts,
		sessions:      sessions,
		tickets:       tickets,
		connectTokens: connectTokens,
	}
}

func (s *AuthService) EnsureReady(ctx context.Context) error {
	return s.accounts.EnsureIndexes(ctx)
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*LoginResult, error) {
	input = normalizeLoginInput(input)
	if err := validateLoginInput(input); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	binding, err := s.accounts.FindOrCreateBinding(ctx, input.Channel, input.Subject, input.DeviceID, now)
	if err != nil {
		return nil, err
	}

	account, err := s.accounts.UpsertAccount(ctx, binding, now)
	if err != nil {
		return nil, err
	}

	sessionID, err := newID("sess", 18)
	if err != nil {
		return nil, err
	}
	ticketID, err := newID("lt", 18)
	if err != nil {
		return nil, err
	}
	connectTokenID, err := newID("ct", 18)
	if err != nil {
		return nil, err
	}

	sessionTTL := s.sessionTTL()
	ticketTTL := s.ticketTTL()
	connectTTL := s.connectTokenTTL()
	session := &Session{
		SessionID:   sessionID,
		AccountID:   account.ID,
		PlayerID:    account.PlayerID,
		Channel:     binding.Channel,
		Subject:     binding.Subject,
		DeviceID:    binding.DeviceID,
		CreatedAt:   now,
		ExpiresAt:   now.Add(sessionTTL),
		LastLoginAt: now,
	}
	if err := s.sessions.SaveSession(ctx, session, sessionTTL); err != nil {
		return nil, err
	}

	ticket := &LoginTicket{
		TicketID:  ticketID,
		SessionID: sessionID,
		AccountID: account.ID,
		CreatedAt: now,
		ExpiresAt: now.Add(ticketTTL),
	}
	if err := s.tickets.Save(ctx, ticket, ticketTTL); err != nil {
		return nil, err
	}
	connectToken := &ConnectToken{
		TokenID:   connectTokenID,
		SessionID: sessionID,
		AccountID: account.ID,
		DeviceID:  binding.DeviceID,
		CreatedAt: now,
		ExpiresAt: now.Add(connectTTL),
	}
	if err := s.connectTokens.Save(ctx, connectToken, connectTTL); err != nil {
		return nil, err
	}

	return &LoginResult{
		AccountID:         account.ID,
		PlayerID:          account.PlayerID,
		SessionID:         sessionID,
		LoginTicket:       ticketID,
		ConnectToken:      connectTokenID,
		SessionExpiresAt:  session.ExpiresAt,
		TicketExpiresAt:   ticket.ExpiresAt,
		ConnectExpiresAt:  connectToken.ExpiresAt,
		SessionTTLSeconds: int64(sessionTTL / time.Second),
		TicketTTLSeconds:  int64(ticketTTL / time.Second),
		ConnectTTLSeconds: int64(connectTTL / time.Second),
	}, nil
}

func (s *AuthService) ConsumeLoginTicket(ctx context.Context, input ConsumeTicketInput) (*Session, error) {
	ticketID := strings.TrimSpace(input.Ticket)
	if ticketID == "" {
		return nil, fmt.Errorf("%w: ticket is required", ErrInvalidArgument)
	}

	ticket, err := s.tickets.Consume(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	session, err := s.sessions.GetSession(ctx, ticket.SessionID)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (s *AuthService) ConsumeConnectToken(ctx context.Context, input ConsumeConnectTokenInput) (*Session, error) {
	tokenID := strings.TrimSpace(input.Token)
	if tokenID == "" {
		return nil, fmt.Errorf("%w: token is required", ErrInvalidArgument)
	}

	token, err := s.connectTokens.Consume(ctx, tokenID)
	if err != nil {
		return nil, err
	}

	session, err := s.sessions.GetSession(ctx, token.SessionID)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (s *AuthService) ValidateSession(ctx context.Context, input ValidateSessionInput) (*Session, error) {
	sessionID := strings.TrimSpace(input.SessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("%w: session_id is required", ErrInvalidArgument)
	}
	return s.sessions.GetSession(ctx, sessionID)
}

func validateLoginInput(input LoginInput) error {
	if input.Channel == "" {
		return fmt.Errorf("%w: channel is required", ErrInvalidArgument)
	}
	if input.DeviceID == "" {
		return fmt.Errorf("%w: device_id is required", ErrInvalidArgument)
	}
	if input.Subject == "" {
		return fmt.Errorf("%w: subject is required", ErrInvalidArgument)
	}
	return nil
}

func normalizeLoginInput(input LoginInput) LoginInput {
	input.Channel = strings.TrimSpace(strings.ToLower(input.Channel))
	input.Subject = strings.TrimSpace(input.Subject)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	if input.Subject == "" && input.Channel == "guest" {
		input.Subject = input.DeviceID
	}
	return input
}

func (s *AuthService) sessionTTL() time.Duration {
	_ = s.cfg
	return defaultSessionTTL
}

func (s *AuthService) ticketTTL() time.Duration {
	return defaultLoginTicketTTL
}

func (s *AuthService) connectTokenTTL() time.Duration {
	return defaultConnectTTL
}

func isPublicError(err error) bool {
	return errors.Is(err, ErrInvalidArgument) ||
		errors.Is(err, ErrSessionNotFound) ||
		errors.Is(err, ErrTicketNotFound) ||
		errors.Is(err, ErrConnectTokenNotFound)
}
