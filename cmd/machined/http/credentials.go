package http

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete"
	"github.com/gorilla/mux"
	"net/http"
)

type credentialsHandler struct {
	credentials libmachete.Credentials
}

func getCredentialID(req *http.Request) string {
	return mux.Vars(req)["key"]
}

func (h *credentialsHandler) getOne(resp http.ResponseWriter, req *http.Request) {
	key := getCredentialID(req)
	cr, err := h.credentials.Get(key)
	if err == nil {
		libmachete.ContentTypeJSON.Respond(resp, cr)
	} else {
		respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown credential:%s", key))
	}
}

func (h *credentialsHandler) getAll(resp http.ResponseWriter, req *http.Request) {
	log.Infoln("List credentials")
	all, err := h.credentials.ListIds()
	if err != nil {
		respondError(http.StatusInternalServerError, resp, err)
		return
	}
	libmachete.ContentTypeJSON.Respond(resp, all)
}

func (h *credentialsHandler) create(resp http.ResponseWriter, req *http.Request) {
	provisioner := req.URL.Query().Get("provisioner")
	key := getCredentialID(req)
	log.Infof("Add credential %v, %v\n", provisioner, key)

	err := h.credentials.CreateCredential(provisioner, key, req.Body, libmachete.ContentTypeJSON)

	if err == nil {
		return
	}

	switch err.Code {
	case libmachete.ErrDuplicate:
		respondError(http.StatusConflict, resp, err)
		return
	case libmachete.ErrNotFound:
		respondError(http.StatusNotFound, resp, err)
		return
	default:
		respondError(http.StatusInternalServerError, resp, err)
		return
	}
}

func (h *credentialsHandler) update(resp http.ResponseWriter, req *http.Request) {
	key := getCredentialID(req)
	log.Infof("Update credential %v\n", key)

	err := h.credentials.UpdateCredential(key, req.Body, libmachete.ContentTypeJSON)

	if err == nil {
		return
	}

	switch err.Code {
	case libmachete.ErrDuplicate:
		respondError(http.StatusConflict, resp, err)
		return
	case libmachete.ErrNotFound:
		respondError(http.StatusNotFound, resp, err)
		return
	default:
		respondError(http.StatusInternalServerError, resp, err)
		return
	}
}

func (h *credentialsHandler) delete(resp http.ResponseWriter, req *http.Request) {
	key := getCredentialID(req)
	err := h.credentials.Delete(key)
	if err != nil {
		respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown credential:%s", key))
		return
	}
}

func (h *credentialsHandler) attachTo(router *mux.Router) {
	router.HandleFunc("/json", h.getAll).Methods("GET")
	router.HandleFunc("/{key}/create", h.create).Methods("POST")
	router.HandleFunc("/{key}", h.update).Methods("PUT")
	router.HandleFunc("/{key}/json", h.getOne).Methods("GET")
	router.HandleFunc("/{key}", h.delete).Methods("DELETE")
}
