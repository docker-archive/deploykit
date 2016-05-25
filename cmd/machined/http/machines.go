package http

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/storage"
	"github.com/gorilla/mux"
	"net/http"
)

type machineHandler struct {
	creds        libmachete.Credentials
	keystore     libmachete.Keys
	templates    libmachete.Templates
	machines     libmachete.Machines
	provisioners libmachete.MachineProvisioners
}

func getMachineID(req *http.Request) string {
	return mux.Vars(req)["key"]
}

func (h *machineHandler) getOne(codec *libmachete.Codec) Handler {
	return func(resp http.ResponseWriter, req *http.Request) {
		key := getMachineID(req)
		mr, err := h.machines.Get(key)
		if err == nil {
			codec.Respond(resp, mr)
		} else {
			respondError(
				http.StatusNotFound,
				resp,
				fmt.Errorf("Unknown machine:%s, err=%s", key, err.Error()))
		}
	}
}

func (h *machineHandler) getAll(resp http.ResponseWriter, req *http.Request) {
	log.Infoln("List machines")
	long := len(req.URL.Query().Get("long")) > 0

	if long {
		all, err := h.machines.List()
		if err != nil {
			respondError(http.StatusInternalServerError, resp, err)
			return
		}
		libmachete.ContentTypeJSON.Respond(resp, all)
		return
	}

	all, err := h.machines.ListIds()
	if err != nil {
		respondError(http.StatusInternalServerError, resp, err)
		return
	}
	libmachete.ContentTypeJSON.Respond(resp, all)
}

func getProvisionControls(req *http.Request) api.ProvisionControls {
	// TODO(wfarner): It may be worth exploring a way for the provisioner to specify the
	// parameters it supports.  Proceeding with this for now for simplicity.
	// Omit query values used in this context for the purposes of the provisioner.
	queryValues := req.URL.Query()
	queryValues.Del("credentials")
	queryValues.Del("template")

	return api.ProvisionControls(queryValues)
}

func (h *machineHandler) create(resp http.ResponseWriter, req *http.Request) {
	credentials := req.URL.Query().Get("credentials")
	template := req.URL.Query().Get("template")

	// TODO - fix this in framework to return default values
	if credentials == "" {
		credentials = "default"
	}
	if template == "" {
		template = "default"
	}

	provisionControls := getProvisionControls(req)

	log.Infof("Add machine controls=%v, template=%v, as %v", provisionControls, template, credentials)

	// credential
	cred, err := h.creds.Get(credentials)
	if err != nil {
		respondError(http.StatusNotFound, resp, err)
		return
	}

	// TODO(wfarner): It's odd that the provisioner name comes from the credentials.  It would seem more appropriate
	// for the credentials to be scoped _within_ provisioners, and the request to directly specify the provisioner
	// to use.
	builder, has := h.provisioners.GetBuilder(cred.ProvisionerName())
	if !has {
		respondError(http.StatusNotFound, resp, err)
		return
	}

	// Load template
	tpl, err := h.templates.Get(storage.TemplateID{Provisioner: builder.Name, Name: template})
	if err != nil {
		respondError(http.StatusNotFound, resp, err)
		return
	}

	provisioner, err := builder.Build(provisionControls, cred)
	if err != nil {
		respondError(http.StatusBadRequest, resp, err)
		return
	}

	events, machineErr := h.machines.CreateMachine(
		provisioner,
		h.keystore,
		cred,
		tpl,
		req.Body,
		libmachete.CodecByContentTypeHeader(req))

	if machineErr == nil {
		// TODO - if the client requests streaming events... send that out here.
		go func() {
			for event := range events {
				log.Infoln("Event:", event)
			}
		}()
		return
	}

	switch machineErr.Code {
	case libmachete.ErrDuplicate:
		respondError(http.StatusConflict, resp, machineErr)
		return
	case libmachete.ErrNotFound:
		respondError(http.StatusNotFound, resp, machineErr)
		return
	default:
		respondError(http.StatusInternalServerError, resp, machineErr)
		return
	}
}

func (h *machineHandler) delete(resp http.ResponseWriter, req *http.Request) {
	machineName := getMachineID(req)
	credentials := req.URL.Query().Get("credentials")

	// TODO - fix this in framework to return default values
	if credentials == "" {
		credentials = "default"
	}

	// TODO(wfarner): ProvisionControls is no longer an appropriate name since it's reused for deletion.  Leaving
	// for now as a revamp is imminent.
	deleteControls := getProvisionControls(req)

	log.Infof("Delete machine %v as %v with controls=%v", machineName, credentials, deleteControls)

	// credential
	cred, err := h.creds.Get(credentials)
	if err != nil {
		respondError(http.StatusBadRequest, resp, err)
		return
	}

	// Load the record of the machine by name
	record, err := h.machines.Get(machineName)
	if err != nil {
		respondError(http.StatusNotFound, resp, err)
		return
	}

	builder, has := h.provisioners.GetBuilder(record.Provisioner)
	if !has {
		respondError(http.StatusNotFound, resp, err)
		return
	}

	provisioner, err := builder.Build(deleteControls, cred)
	if err != nil {
		respondError(http.StatusBadRequest, resp, err)
		return
	}

	events, machineErr := h.machines.DeleteMachine(provisioner, h.keystore, cred, record)
	if machineErr == nil {
		// TODO - if the client requests streaming events... send that out here.
		go func() {
			for event := range events {
				log.Infoln("Event:", event)
			}
		}()
		return
	}

	switch machineErr.Code {
	case libmachete.ErrDuplicate:
		respondError(http.StatusConflict, resp, machineErr)
		return
	case libmachete.ErrNotFound:
		respondError(http.StatusNotFound, resp, machineErr)
		return
	default:
		respondError(http.StatusInternalServerError, resp, machineErr)
		return
	}
}

func (h *machineHandler) attachTo(router *mux.Router) {
	router.HandleFunc("/json", h.getAll).Methods("GET")
	router.HandleFunc("/create", h.create).Methods("POST")
	router.HandleFunc("/{key}/json", h.getOne(libmachete.ContentTypeJSON)).Methods("GET")
	router.HandleFunc("/{key}/yaml", h.getOne(libmachete.ContentTypeYAML)).Methods("GET")
	router.HandleFunc("/{key}", h.delete).Methods("DELETE")
}
