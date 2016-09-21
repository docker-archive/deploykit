package instance

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/instance"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
)

type instanceHandler struct {
	provisioner instance.Plugin
}

func getInstanceID(req *http.Request) instance.ID {
	return instance.ID(mux.Vars(req)["key"])
}

func (h *instanceHandler) describe(req *http.Request) (interface{}, error) {
	tags := map[string]string{}
	for key, values := range req.URL.Query() {
		for _, value := range values {
			tags[key] = value
			break
		}
	}

	return h.provisioner.DescribeInstances(tags)
}

func (h *instanceHandler) provision(req *http.Request) (interface{}, error) {
	buff, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, spi.NewError(spi.ErrBadInput, fmt.Sprintf("Failed to read request input: %s", err))
	}

	request := ProvisionRequest{}
	err = json.Unmarshal(buff, &request)
	if err != nil {
		return nil, spi.NewError(spi.ErrUnknown, fmt.Sprintf("Failed to unmarshal response: %s", err))
	}

	return h.provisioner.Provision(instance.Spec{
		Properties:       *request.Request,
		Tags:             request.Tags,
		InitScript:       request.InitScript,
		PrivateIPAddress: request.PrivateIP,
		Volume:           request.Volume,
	})
}

func (h *instanceHandler) destroy(req *http.Request) (interface{}, error) {
	return nil, h.provisioner.Destroy(getInstanceID(req))
}

func (h *instanceHandler) attachTo(router *mux.Router) {
	router.HandleFunc("/", OutputHandler(h.describe)).Methods("GET")
	router.HandleFunc("/", OutputHandler(h.provision)).Methods("POST")
	router.HandleFunc("/{key}", OutputHandler(h.destroy)).Methods("DELETE")
}
