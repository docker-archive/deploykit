package server

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/server/api"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/group"
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
	gid := req.URL.Query().Get("group")
	if len(gid) == 0 {
		return nil, spi.NewError(spi.ErrBadInput, "Group must be specified")
	}

	return h.provisioner.DescribeInstances(group.ID(gid))
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

	return h.provisioner.Provision(request.Group, string(*request.Request), request.Volume)
}

func (h *instanceHandler) destroy(req *http.Request) (interface{}, error) {
	return nil, h.provisioner.Destroy(getInstanceID(req))
}

func (h *instanceHandler) attachTo(router *mux.Router) {
	router.HandleFunc("/", OutputHandler(h.describe)).Methods("GET")
	router.HandleFunc("/", OutputHandler(h.provision)).Methods("POST")
	router.HandleFunc("/{key}", OutputHandler(h.destroy)).Methods("DELETE")
}
