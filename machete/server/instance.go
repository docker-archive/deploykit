package server

import (
	"github.com/docker/libmachete/api"
	"github.com/docker/libmachete/machete/spi"
	"github.com/docker/libmachete/machete/spi/instance"
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

func (h *instanceHandler) listGroup(req *http.Request) (interface{}, *spi.Error) {
	group := req.URL.Query().Get("group")
	if len(group) == 0 {
		return nil, &spi.Error{Code: api.ErrBadInput, Message: "Group must be specified"}
	}

	return h.provisioner.ListGroup(instance.GroupID(group))
}

func (h *instanceHandler) provision(req *http.Request) (interface{}, *spi.Error) {
	buff, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, &spi.Error{Code: api.ErrBadInput, Message: "Failed to read request input"}
	}

	return h.provisioner.Provision(string(buff))
}

func (h *instanceHandler) destroy(req *http.Request) (interface{}, *spi.Error) {
	return nil, h.provisioner.Destroy(getInstanceID(req))
}

func (h *instanceHandler) attachTo(router *mux.Router) {
	router.HandleFunc("/", outputHandler(h.listGroup)).Methods("GET")
	router.HandleFunc("/", outputHandler(h.provision)).Methods("POST")
	router.HandleFunc("/{key}", outputHandler(h.destroy)).Methods("DELETE")
}
