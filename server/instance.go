package server

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/server/api"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/instance"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
)

type instanceHandler struct {
	provisioner instance.Provisioner
}

func getInstanceID(req *http.Request) instance.ID {
	return instance.ID(mux.Vars(req)["key"])
}

func (h *instanceHandler) describe(req *http.Request) (interface{}, error) {
	group := req.URL.Query().Get("group")
	if len(group) == 0 {
		return nil, spi.NewError(spi.ErrBadInput, "Group must be specified")
	}

	return h.provisioner.DescribeInstances(instance.GroupID(group))
}

func (h *instanceHandler) provision(req *http.Request) (interface{}, error) {
	buff, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, spi.NewError(spi.ErrBadInput, fmt.Sprintf("Failed to read request input: %s", err))
	}

	request := api.ProvisionRequest{}
	err = json.Unmarshal(buff, &request)
	if err != nil {
		return nil, spi.NewError(spi.ErrUnknown, fmt.Sprintf("Failed to unmarshal response: %s", err))
	}

	return h.provisioner.Provision(string(*request.Request), request.Volume)
}

func (h *instanceHandler) destroy(req *http.Request) (interface{}, error) {
	return nil, h.provisioner.Destroy(getInstanceID(req))
}

func (h *instanceHandler) attachTo(router *mux.Router) {
	router.HandleFunc("/", outputHandler(h.describe)).Methods("GET")
	router.HandleFunc("/", outputHandler(h.provision)).Methods("POST")
	router.HandleFunc("/{key}", outputHandler(h.destroy)).Methods("DELETE")
}
