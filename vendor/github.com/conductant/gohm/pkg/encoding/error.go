package encoding

import (
	"errors"
)

var (
	ErrUnknownContentType = errors.New("error-no-content-type")
	ErrIncompatibleType   = errors.New("error-incompatible-type")
	ErrBadContentType     = errors.New("err-bad-content-type")
)
