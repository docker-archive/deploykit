package http

import (
	"github.com/docker/libmachete/api"
	"github.com/gorilla/mux"
	"net/http"
)

type credentialsHandler struct {
	credentials api.Credentials
}

func getCredentialID(req *http.Request) api.CredentialsID {
	vars := mux.Vars(req)
	return api.CredentialsID{Provisioner: vars["provisioner"], Name: vars["key"]}
}

func (h *credentialsHandler) getOne(req *http.Request) (interface{}, *api.Error) {
	return h.credentials.Get(getCredentialID(req))
}

func (h *credentialsHandler) getAll(req *http.Request) (interface{}, *api.Error) {
	return h.credentials.ListIds()
}

func (h *credentialsHandler) create(req *http.Request) (interface{}, *api.Error) {
	return nil, h.credentials.CreateCredential(getCredentialID(req), req.Body, api.ContentTypeJSON)
}

func (h *credentialsHandler) update(req *http.Request) (interface{}, *api.Error) {
	return nil, h.credentials.UpdateCredential(getCredentialID(req), req.Body, api.ContentTypeJSON)
}

func (h *credentialsHandler) delete(req *http.Request) (interface{}, *api.Error) {
	return nil, h.credentials.Delete(getCredentialID(req))
}

func (h *credentialsHandler) attachTo(router *mux.Router) {
	router.HandleFunc("/json", outputHandler(h.getAll)).Methods("GET")
	router.HandleFunc("/{provisioner}/{key}/create", outputHandler(h.create)).Methods("POST")
	router.HandleFunc("/{provisioner}/{key}", outputHandler(h.update)).Methods("PUT")
	router.HandleFunc("/{provisioner}/{key}/json", outputHandler(h.getOne)).Methods("GET")
	router.HandleFunc("/{provisioner}/{key}", outputHandler(h.delete)).Methods("DELETE")
}
