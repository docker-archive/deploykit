package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var (
	ErrMissingInput                 = errors.New("err-missing-input")
	ErrUnknownMethod                = errors.New("err-unknown-method")
	ErrNotSupportedUrlParameterType = errors.New("err-not-supported-url-query-param-type")
	ErrNoHttpHeaderSpec             = errors.New("err-no-http-header-spec")
	ErrNoSignKey                    = errors.New("err-no-sign-key")
	ErrNoVerifyKey                  = errors.New("err-no-verify-key")
	ErrInvalidAuthToken             = errors.New("err-invalid-token")
	ErrExpiredAuthToken             = errors.New("err-token-expired")
	ErrNoAuthToken                  = errors.New("err-no-auth-token")
	ErrBadContentType               = errors.New("err-bad-content-type")
)

var (
	DefaultErrorRenderer = func(resp http.ResponseWriter, req *http.Request, message string, code int) error {
		resp.WriteHeader(code)
		escaped := message
		if len(message) > 0 {
			escaped = strings.Replace(message, "\"", "'", -1)
		}
		// First look for accept content type in the header
		ct := content_type_for_response(req)
		switch ct {
		case "application/json":
			resp.Write([]byte(fmt.Sprintf("{ \"error\": \"%s\" }", escaped)))
		case "application/protobuf":
		default:
			resp.Write([]byte(fmt.Sprintf("<html><body>Error: %s </body></html>", escaped)))
		}
		return nil
	}
)
