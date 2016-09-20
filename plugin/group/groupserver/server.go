package groupserver

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/group"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"reflect"
	"runtime/debug"
)

type httpAdapter struct {
	plugin group.Plugin
}

func getGroupID(req *http.Request) group.ID {
	return group.ID(mux.Vars(req)["id"])
}

func getConfiguration(req *http.Request) (group.Configuration, error) {
	grp := group.Configuration{}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return grp, fmt.Errorf("Failed to read body: %s", err)
	}

	err = json.Unmarshal(body, &grp)
	if err != nil {
		return grp, spi.NewError(spi.ErrBadInput, "Invalid group configuration: %s", err)
	}

	return grp, nil
}

func (h httpAdapter) watch(req *http.Request) (interface{}, error) {
	config, err := getConfiguration(req)
	if err != nil {
		return nil, err
	}

	return nil, h.plugin.WatchGroup(config)
}

func (h httpAdapter) unwatch(req *http.Request) (interface{}, error) {
	id := getGroupID(req)
	if len(id) == 0 {
		return nil, spi.NewError(spi.ErrBadInput, "Group ID must not be blank")
	}

	return nil, h.plugin.UnwatchGroup(id)
}

func (h httpAdapter) inspect(req *http.Request) (interface{}, error) {
	id := getGroupID(req)
	if len(id) == 0 {
		return nil, spi.NewError(spi.ErrBadInput, "Group ID must not be blank")
	}

	desc, err := h.plugin.InspectGroup(id)
	if err != nil {
		return nil, err
	}

	return &desc, nil
}

func (h httpAdapter) describeUpdate(req *http.Request) (interface{}, error) {
	config, err := getConfiguration(req)
	if err != nil {
		return nil, err
	}

	desc, err := h.plugin.DescribeUpdate(config)
	if err != nil {
		return nil, err
	}

	return &desc, nil
}

func (h httpAdapter) updateGroup(req *http.Request) (interface{}, error) {
	config, err := getConfiguration(req)
	if err != nil {
		return nil, err
	}

	return nil, h.plugin.UpdateGroup(config)
}

func (h httpAdapter) destroyGroup(req *http.Request) (interface{}, error) {
	id := getGroupID(req)
	if len(id) == 0 {
		return nil, spi.NewError(spi.ErrBadInput, "Group ID must not be blank")
	}

	return nil, h.plugin.DestroyGroup(id)
}

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

type simpleHandler func(req *http.Request) (interface{}, error)

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

func outputHandler(handler simpleHandler) Handler {
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
