package http

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/api"
	"net/http"
)

// Handler is shorthand for an HTTP request handler function.
type Handler func(resp http.ResponseWriter, req *http.Request)

var errorCodeMap = map[int]int{
	api.ErrBadInput:  http.StatusBadRequest,
	api.ErrUnknown:   http.StatusInternalServerError,
	api.ErrDuplicate: http.StatusConflict,
	api.ErrNotFound:  http.StatusNotFound,
}

// SimpleHandler is a reduced HTTP handler interface that may be used with handleError().
type SimpleHandler func(req *http.Request) (interface{}, *api.Error)

func handleError(err api.Error, resp http.ResponseWriter) {
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
