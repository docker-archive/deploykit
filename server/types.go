package server

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/instance"
	"github.com/spf13/pflag"
	"net/http"
)

// Handler is shorthand for an HTTP request handler function.
type Handler func(resp http.ResponseWriter, req *http.Request)

var errorCodeMap = map[int]int{
	spi.ErrBadInput:  http.StatusBadRequest,
	spi.ErrUnknown:   http.StatusInternalServerError,
	spi.ErrDuplicate: http.StatusConflict,
	spi.ErrNotFound:  http.StatusNotFound,
}

// SimpleHandler is a reduced HTTP handler interface that may be used with handleError().
type SimpleHandler func(req *http.Request) (interface{}, *spi.Error)

func respondError(code int, resp http.ResponseWriter, err error) {
	resp.WriteHeader(code)
	resp.Header().Set("Content-Type", "application/json")
	body, err := json.Marshal(map[string]string{"error": err.Error()})
	if err == nil {
		resp.Write(body)
		return
	}
	panic(fmt.Sprintf("Failed to marshal error text: %s", err.Error()))
}

func handleError(err spi.Error, resp http.ResponseWriter) {
	statusCode, mapped := errorCodeMap[err.Code]
	if !mapped {
		statusCode = http.StatusInternalServerError
	}

	respondError(statusCode, resp, err)
}

func outputHandler(handler SimpleHandler) Handler {
	return func(resp http.ResponseWriter, req *http.Request) {
		// Handle panics cleanly.
		defer func() {
			if err := recover(); err != nil {
				respondError(500, resp, fmt.Errorf("Panic: %v", err))
			}
		}()

		result, err := handler(req)
		if err == nil {
			if req.Method == "POST" {
				resp.WriteHeader(http.StatusCreated)
			} else {
				resp.WriteHeader(http.StatusOK)
			}

			buff, err := json.Marshal(result)
			if err != nil {
				resp.WriteHeader(http.StatusInternalServerError)
				resp.Write([]byte(err.Error()))
				return
			}
			resp.Header().Set("Content-Type", "application/json")
			resp.Write(buff)
		} else {
			handleError(*err, resp)
		}
	}
}

// ProvisionerBuilder allows a provider to define options and available provisioner types.
type ProvisionerBuilder interface {
	Flags() *pflag.FlagSet

	BuildInstanceProvisioner() (instance.Provisioner, error)
}
