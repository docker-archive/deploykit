package http

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/api"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/gorilla/mux"
	"net/http"
)

type machineHandler struct {
	creds        api.Credentials
	keystore     api.SSHKeys
	templates    api.Templates
	machines     api.Machines
	provisioners api.MachineProvisioners
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

func (h *machineHandler) getProvisionerBuilder(req *http.Request) (*api.ProvisionerBuilder, *api.Error) {
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

func (h *machineHandler) credentialsOrDefault(
	provisioner string,
	req *http.Request,
	defaultName string) (spi.Credential, *api.Error) {

	cred, err := h.creds.Get(api.CredentialsID{
		Provisioner: provisioner,
		Name:        orDefault(req.URL.Query().Get("credentials"), defaultName)})
	if err != nil {
		return nil, err
	}
	return cred, nil
}

func (h *machineHandler) templateOrDefault(
	provisioner string,
	req *http.Request,
	defaultName string) (spi.MachineRequest, *api.Error) {

	template, apiErr := h.templates.Get(api.TemplateID{
		Provisioner: provisioner,
		Name:        orDefault(req.URL.Query().Get("template"), defaultName)})
	if apiErr != nil {
		// Overriding the error code here as ErrNotFound should not be returned for a referenced auxiliary
		// entity.
		return nil, &api.Error{api.ErrBadInput, apiErr.Error()}
	}
	return template, nil
}

func (h *machineHandler) create(req *http.Request) (interface{}, *api.Error) {
	builder, apiErr := h.getProvisionerBuilder(req)
	if apiErr != nil {
		return nil, apiErr
	}

	cred, apiErr := h.credentialsOrDefault(builder.Name, req, "default")
	if apiErr != nil {
		return nil, apiErr
	}

	template, apiErr := h.templateOrDefault(builder.Name, req, "default")
	if apiErr != nil {
		return nil, apiErr
	}

	provisioner, err := builder.Build(getProvisionControls(req), cred)
	if err != nil {
		return nil, &api.Error{api.ErrBadInput, err.Error()}
	}

	events, apiErr := h.machines.CreateMachine(
		provisioner,
		h.keystore,
		cred,
		template,
		req.Body,
		api.ContentTypeJSON)

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

func (h *machineHandler) delete(req *http.Request) (interface{}, *api.Error) {
	cred, apiErr := h.credentialsOrDefault(provisionerNameFromQuery(req), req, "default")
	if apiErr != nil {
		return nil, apiErr
	}

	// TODO(wfarner): ProvisionControls is no longer an appropriate name since it's reused for deletion.  Leaving
	// for now as a revamp is imminent.
	deleteControls := getProvisionControls(req)

	// Load the record of the machine by name
	record, apiErr := h.machines.Get(getMachineID(req))
	if apiErr != nil {
		return nil, apiErr
	}

	builder, has := h.provisioners.GetBuilder(record.Provisioner)
	if !has {
		return nil, &api.Error{
			api.ErrUnknown,
			fmt.Sprintf("Machine record referenced a provisioner that does not exist: %s", record.Provisioner)}
	}

	provisioner, err := builder.Build(deleteControls, cred)
	if err != nil {
		return nil, &api.Error{api.ErrBadInput, err.Error()}
	}

	events, apiErr := h.machines.DeleteMachine(provisioner, h.keystore, cred, record)
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
