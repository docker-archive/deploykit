package http

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/api"
	"github.com/docker/libmachete/machines"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/gorilla/mux"
	"net/http"
)

type machineHandler struct {
	creds        api.Credentials
	keystore     api.SSHKeys
	templates    api.Templates
	machines     api.Machines
	provisioners machines.MachineProvisioners
}

func getMachineID(req *http.Request) api.MachineID {
	return api.MachineID(mux.Vars(req)["key"])
}

func (h *machineHandler) getOne(req *http.Request) (interface{}, *api.Error) {
	return h.machines.Get(getMachineID(req))
}

func (h *machineHandler) getAll(req *http.Request) (interface{}, *api.Error) {
	if len(req.URL.Query().Get("long")) > 0 {
		return h.machines.List()
	}

	return h.machines.ListIds()
}

func getProvisionControls(req *http.Request) spi.ProvisionControls {
	// TODO(wfarner): It may be worth exploring a way for the provisioner to specify the
	// parameters it supports.  Proceeding with this for now for simplicity.
	// Omit query values used in this context for the purposes of the provisioner.
	queryValues := req.URL.Query()
	queryValues.Del("credentials")
	queryValues.Del("template")

	return spi.ProvisionControls(queryValues)
}

func provisionerNameFromQuery(req *http.Request) string {
	return req.URL.Query().Get("provisioner")
}

func (h *machineHandler) getProvisionerBuilder(req *http.Request) (*machines.ProvisionerBuilder, *api.Error) {
	provisionerName := provisionerNameFromQuery(req)
	builder, has := h.provisioners.GetBuilder(provisionerName)
	if !has {
		return nil, &api.Error{
			api.ErrBadInput,
			fmt.Sprintf("Unknown provisioner: %s", provisionerName)}
	}
	return &builder, nil
}

func orDefault(v string, defaultValue string) string {
	if v == "" {
		return defaultValue
	}
	return v
}

func blockingRequested(req *http.Request) bool {
	return len(req.URL.Query().Get("block")) > 0
}

func (h *machineHandler) create(req *http.Request) (interface{}, *api.Error) {
	events, apiErr := h.machines.CreateMachine(
		provisionerNameFromQuery(req),
		orDefault(req.URL.Query().Get("credentials"), "default"),
		getProvisionControls(req),
		orDefault(req.URL.Query().Get("template"), "default"),
		req.Body,
		api.ContentTypeJSON)
	if apiErr == nil {
		readEvents := func() {
			for event := range events {
				log.Infoln("Event:", event)
			}
		}

		if blockingRequested(req) {
			// TODO - if the client requests streaming events... send that out here.
			readEvents()
		} else {
			go func() {
				readEvents()
			}()
		}

		return nil, nil
	}

	return nil, apiErr
}

func (h *machineHandler) delete(req *http.Request) (interface{}, *api.Error) {
	events, apiErr := h.machines.DeleteMachine(
		orDefault(req.URL.Query().Get("credentials"), "default"),
		getProvisionControls(req),
		getMachineID(req))
	if apiErr == nil {
		readEvents := func() {
			for event := range events {
				log.Infoln("Event:", event)
			}
		}

		if blockingRequested(req) {
			// TODO - if the client requests streaming events... send that out here.
			readEvents()
		} else {
			go func() {
				readEvents()
			}()
		}
	}

	return nil, apiErr
}

func (h *machineHandler) attachTo(router *mux.Router) {
	router.HandleFunc("/", outputHandler(h.getAll)).Methods("GET")
	router.HandleFunc("/{key}", outputHandler(h.create)).Methods("POST")
	router.HandleFunc("/{key}", outputHandler(h.getOne)).Methods("GET")
	router.HandleFunc("/{key}", outputHandler(h.delete)).Methods("DELETE")
}
