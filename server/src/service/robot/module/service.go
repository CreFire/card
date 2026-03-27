package module

import (
	"context"
	"fmt"
	"strings"
)

const defaultRemoteAddr = "robot://local"

type RobotService struct {
	authClient AuthClient
	gateClient GateClient
}

func NewRobotService(authClient AuthClient, gateClient GateClient) *RobotService {
	return &RobotService{
		authClient: authClient,
		gateClient: gateClient,
	}
}

func (s *RobotService) AuthLogin(ctx context.Context, input AuthLoginInput) (*AuthLoginResult, error) {
	input = normalizeAuthLoginInput(input)
	if err := validateAuthLoginInput(input); err != nil {
		return nil, err
	}
	return s.authClient.Login(ctx, input)
}

func (s *RobotService) LoginCase(ctx context.Context, input LoginCaseInput) (*LoginCaseResult, error) {
	authInput := AuthLoginInput{
		Channel:  input.Channel,
		Subject:  input.Subject,
		DeviceID: input.DeviceID,
	}

	auth, err := s.AuthLogin(ctx, authInput)
	if err != nil {
		return nil, err
	}

	remoteAddr := strings.TrimSpace(input.RemoteAddr)
	if remoteAddr == "" {
		remoteAddr = defaultRemoteAddr
	}

	opened, err := s.gateClient.OpenConnection(ctx, OpenConnectionInput{
		RemoteAddr: remoteAddr,
	})
	if err != nil {
		return nil, err
	}

	bound, err := s.gateClient.LoginByToken(ctx, GateLoginByTokenInput{
		ConnID: opened.ConnID,
		Token:  auth.ConnectToken,
	})
	if err != nil {
		return nil, err
	}

	return &LoginCaseResult{
		Auth:   auth,
		Opened: opened,
		Bound:  bound,
	}, nil
}

func normalizeAuthLoginInput(input AuthLoginInput) AuthLoginInput {
	input.Channel = strings.TrimSpace(strings.ToLower(input.Channel))
	input.Subject = strings.TrimSpace(input.Subject)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	if input.Subject == "" && input.Channel == "guest" {
		input.Subject = input.DeviceID
	}
	return input
}

func validateAuthLoginInput(input AuthLoginInput) error {
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
