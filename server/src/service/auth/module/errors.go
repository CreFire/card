package module

import "errors"

var (
	ErrInvalidArgument      = errors.New("invalid argument")
	ErrSessionNotFound      = errors.New("session not found")
	ErrTicketNotFound       = errors.New("login ticket not found")
	ErrConnectTokenNotFound = errors.New("connect token not found")
)
