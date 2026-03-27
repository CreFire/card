package module

import "errors"

var (
	ErrInvalidArgument = errors.New("invalid argument")
	ErrConnNotFound    = errors.New("connection not found")
)
