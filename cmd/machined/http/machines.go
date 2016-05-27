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
	templates    libmachete.Templates
	machines     libmachete.Machines
	provisioners libmachete.MachineProvisioners
}

func getMachineID(req *http.Request) string {
	return mux.Vars(req)["key"]
}

func (h *machineHandler) getOne(req *http.Request) (interface{}, *libmachete.Error) {
	return h.machines.Get(getMachineID(req))
}

func (h *machineHandler) getAll(req *http.Request) (interface{}, *libmachete.Error) {
	if len(req.URL.Query().Get("long")) > 0 {
		return h.machines.List()
	}

	return h.machines.ListIds()
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

func (h *machineHandler) create(req *http.Request) (interface{}, *libmachete.Error) {
	credentials := req.URL.Query().Get("credentials")
	templateName := req.URL.Query().Get("template")

	// TODO: fix this in framework to return default values
	if credentials == "" {
		credentials = "default"
	}
	if templateName == "" {
		templateName = "default"
	}

	provisionControls := getProvisionControls(req)

	log.Infof("Add machine controls=%v, template=%v, as %v", provisionControls, templateName, credentials)

	// credential
	cred, apiErr := h.creds.Get(credentials)
	if apiErr != nil {
		return nil, apiErr
	}

	// TODO(wfarner): It's odd that the provisioner name comes from the credentials.  It would seem more appropriate
	// for the credentials to be scoped _within_ provisioners, and the request to directly specify the provisioner
	// to use.
	builder, has := h.provisioners.GetBuilder(cred.ProvisionerName())
	if !has {
		return nil, &libmachete.Error{
			libmachete.ErrUnknown,
			fmt.Sprintf("Credentials referenced a provisioner that does not exist: %s", cred.ProvisionerName())}
	}

	// Load template
	template, apiErr := h.templates.Get(storage.TemplateID{Provisioner: builder.Name, Name: templateName})
	if apiErr != nil {
		// Overriding the error code here as ErrNotFound should not be returned for a referenced auxiliary
		// entity.
		return nil, &libmachete.Error{libmachete.ErrBadInput, apiErr.Error()}
	}

	provisioner, err := builder.Build(provisionControls, cred)
	if err != nil {
		return nil, &libmachete.Error{libmachete.ErrBadInput, err.Error()}
	}

	events, apiErr := h.machines.CreateMachine(
		provisioner,
		cred,
		template,
		req.Body,
		libmachete.ContentTypeJSON)

	if apiErr == nil {
		// TODO - if the client requests streaming events... send that out here.
		go func() {
			for event := range events {
				log.Infoln("Event:", event)
			}
		}()
		return nil, nil
	}

	return nil, apiErr
}

func (h *machineHandler) delete(req *http.Request) (interface{}, *libmachete.Error) {
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
	cred, apiErr := h.creds.Get(credentials)
	if apiErr != nil {
		return nil, &libmachete.Error{libmachete.ErrBadInput, apiErr.Error()}
	}

	// Load the record of the machine by name
	record, apiErr := h.machines.Get(machineName)
	if apiErr != nil {
		return nil, apiErr
	}

	builder, has := h.provisioners.GetBuilder(record.Provisioner)
	if !has {
		return nil, &libmachete.Error{
			libmachete.ErrUnknown,
			fmt.Sprintf("Machine record referenced a provisioner that does not exist: %s", record.Provisioner)}
	}

	provisioner, err := builder.Build(deleteControls, cred)
	if err != nil {
		return nil, &libmachete.Error{libmachete.ErrBadInput, err.Error()}
	}

	events, apiErr := h.machines.DeleteMachine(provisioner, cred, record)
	if apiErr == nil {
		// TODO - if the client requests streaming events... send that out here.
		go func() {
			for event := range events {
				log.Infoln("Event:", event)
			}
		}()
		return nil, nil
	}

	return nil, apiErr
}

func (h *machineHandler) attachTo(router *mux.Router) {
	router.HandleFunc("/json", outputHandler(h.getAll)).Methods("GET")
	router.HandleFunc("/create", outputHandler(h.create)).Methods("POST")
	router.HandleFunc("/{key}/json", outputHandler(h.getOne)).Methods("GET")
	router.HandleFunc("/{key}", outputHandler(h.delete)).Methods("DELETE")
}
