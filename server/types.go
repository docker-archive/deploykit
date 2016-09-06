package server

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/instance"
	"github.com/spf13/pflag"
	"net/http"
	"reflect"
	"runtime/debug"
)

// Handler is shorthand for an HTTP request handler function.
type Handler func(resp http.ResponseWriter, req *http.Request)

// Counterpart to the inverse map on the client side.
var spiErrorToHTTPStatus = map[int]int{
	spi.ErrBadInput:  http.StatusBadRequest,
	spi.ErrUnknown:   http.StatusInternalServerError,
	spi.ErrDuplicate: http.StatusConflict,
	spi.ErrNotFound:  http.StatusNotFound,
}

func getStatusCode(err error) int {
	status, mapped := spiErrorToHTTPStatus[spi.CodeFromError(err)]
	if !mapped {
		status = http.StatusInternalServerError
	}
	return status
}

// SimpleHandler is a reduced HTTP handler interface that may be used with handleError().
type SimpleHandler func(req *http.Request) (interface{}, error)

func sendResponse(status int, body interface{}, resp http.ResponseWriter) {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		status = http.StatusInternalServerError
		bodyJSON = []byte(`{"error": "Internal error"`)
		log.Warn("Failed to marshal response body %v: %s", body, err.Error())
	}

	resp.WriteHeader(status)
	resp.Header().Set("Content-Type", "application/json")
	resp.Write(bodyJSON)
}

func errorBody(err error) interface{} {
	return map[string]string{"error": err.Error()}
}

// OutputHandler is a convenience function to provide standard HTTP response behavior.
func OutputHandler(handler SimpleHandler) Handler {
	return func(resp http.ResponseWriter, req *http.Request) {
		// Handle panics cleanly.
		defer func() {
			if err := recover(); err != nil {
				log.Errorf("%s: %s", err, debug.Stack())
				sendResponse(
					http.StatusInternalServerError,
					errorBody(fmt.Errorf("Panic: %s", err)),
					resp)
			}
		}()

		responseBody, err := handler(req)

		var status int
		if err == nil {
			switch req.Method {
			case "POST":
				status = http.StatusCreated
			default:
				status = http.StatusOK
			}
		} else {
			log.Warn("Request failed: ", err)
			status = getStatusCode(err)

			// Only use the error to define the response body if there was no result from the handler.
			if responseBody == nil || reflect.ValueOf(responseBody).IsNil() {
				// Use the error to define the response
				responseBody = errorBody(err)
			}
		}

		sendResponse(status, responseBody, resp)
	}
}

// ProvisionerBuilder allows a provider to define options and available provisioner types.
type ProvisionerBuilder interface {
	Flags() *pflag.FlagSet

	BuildInstanceProvisioner(cluster spi.ClusterID) (instance.Plugin, error)
}
