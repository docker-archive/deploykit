package auth

import (
	"errors"
)

var (
	ErrNoPublicKeyFunc  = errors.New("no-public-key-func")
	ErrNoPrivateKeyFunc = errors.New("no-private-key-func")
	ErrInvalidAuthToken = errors.New("invalid-token")
	ErrExpiredAuthToken = errors.New("token-expired")
	ErrNoAuthToken      = errors.New("no-auth-token")
)
