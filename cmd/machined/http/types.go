package http

import (
	"github.com/docker/libmachete"
	"net/http"
)

// Handler is shorthand for an HTTP request handler function.
type Handler func(resp http.ResponseWriter, req *http.Request)

var errorCodeMap = map[int]int{
	libmachete.ErrBadInput:  http.StatusBadRequest,
	libmachete.ErrUnknown:   http.StatusInternalServerError,
	libmachete.ErrDuplicate: http.StatusConflict,
	libmachete.ErrNotFound:  http.StatusNotFound,
}

// SimpleHandler is a reduced HTTP handler interface that may be used with handleError().
type SimpleHandler func(req *http.Request) (interface{}, *libmachete.Error)

func handleError(err libmachete.Error, resp http.ResponseWriter) {
	statusCode, mapped := errorCodeMap[err.Code]
	if !mapped {
		statusCode = http.StatusInternalServerError
	}

	respondError(statusCode, resp, err)
}

func outputHandler(handler SimpleHandler) Handler {
	return func(resp http.ResponseWriter, req *http.Request) {
		result, err := handler(req)
		if result != nil {
			libmachete.ContentTypeJSON.Respond(resp, result)
		}

		if err != nil {
			handleError(*err, resp)
		}
	}
}
