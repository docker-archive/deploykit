package http

import (
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/storage"
	"github.com/gorilla/mux"
	"net/http"
)

type credentialsHandler struct {
	credentials libmachete.Credentials
}

func getCredentialID(req *http.Request) storage.CredentialsID {
	vars := mux.Vars(req)
	return storage.CredentialsID{Provisioner: vars["provisioner"], Name: vars["key"]}
}

func (h *credentialsHandler) getOne(req *http.Request) (interface{}, *libmachete.Error) {
	return h.credentials.Get(getCredentialID(req))
}

func (h *credentialsHandler) getAll(req *http.Request) (interface{}, *libmachete.Error) {
	return h.credentials.ListIds()
}

func (h *credentialsHandler) create(req *http.Request) (interface{}, *libmachete.Error) {
	return nil, h.credentials.CreateCredential(getCredentialID(req), req.Body, libmachete.ContentTypeJSON)
}

func (h *credentialsHandler) update(req *http.Request) (interface{}, *libmachete.Error) {
	return nil, h.credentials.UpdateCredential(getCredentialID(req), req.Body, libmachete.ContentTypeJSON)
}

func (h *credentialsHandler) delete(req *http.Request) (interface{}, *libmachete.Error) {
	return nil, h.credentials.Delete(getCredentialID(req))
}

func (h *credentialsHandler) attachTo(router *mux.Router) {
	router.HandleFunc("/json", outputHandler(h.getAll)).Methods("GET")
	router.HandleFunc("/{key}/create", outputHandler(h.create)).Methods("POST")
	router.HandleFunc("/{key}", outputHandler(h.update)).Methods("PUT")
	router.HandleFunc("/{key}/json", outputHandler(h.getOne)).Methods("GET")
	router.HandleFunc("/{key}", outputHandler(h.delete)).Methods("DELETE")
}
