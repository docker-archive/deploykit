package http

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/storage"
	"github.com/gorilla/mux"
	"net/http"
)

type templatesHandler struct {
	templates libmachete.Templates
}

func (h *templatesHandler) getOne(resp http.ResponseWriter, req *http.Request) {
	id := getTemplateID(req)
	template, err := h.templates.Get(id)
	if err != nil {
		respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown template:%s", id.Name))
		return
	}
	libmachete.ContentTypeJSON.Respond(resp, template)
}

func (h *templatesHandler) getBlank(resp http.ResponseWriter, req *http.Request) {
	provisioner := getProvisionerName(req)
	log.Infof("Get template example %v", provisioner)

	example, err := h.templates.NewBlankTemplate(provisioner)
	if err != nil {
		respondError(http.StatusNotFound, resp, err)
		return
	}
	libmachete.ContentTypeJSON.Respond(resp, example)
}

func (h *templatesHandler) getAll(resp http.ResponseWriter, req *http.Request) {
	log.Infoln("List templates")
	all, err := h.templates.ListIds()
	if err != nil {
		respondError(http.StatusInternalServerError, resp, err)
		return
	}
	libmachete.ContentTypeJSON.Respond(resp, all)
}

var errorCodeMap = map[int]int{
	libmachete.ErrBadInput:  http.StatusBadRequest,
	libmachete.ErrUnknown:   http.StatusInternalServerError,
	libmachete.ErrDuplicate: http.StatusConflict,
	libmachete.ErrNotFound:  http.StatusNotFound,
}

func handleError(resp http.ResponseWriter, err libmachete.Error) {
	statusCode, mapped := errorCodeMap[err.Code]
	if !mapped {
		statusCode = http.StatusInternalServerError
	}

	respondError(statusCode, resp, err)
}

func (h *templatesHandler) create(resp http.ResponseWriter, req *http.Request) {
	id := getTemplateID(req)
	log.Infof("Add template %v", id)

	err := h.templates.CreateTemplate(id, req.Body, libmachete.ContentTypeJSON)

	if err != nil {
		handleError(resp, *err)
	}
}

func (h *templatesHandler) update(resp http.ResponseWriter, req *http.Request) {
	id := getTemplateID(req)
	log.Infof("Update template %v", id.Name)

	err := h.templates.UpdateTemplate(id, req.Body, libmachete.ContentTypeJSON)

	if err != nil {
		handleError(resp, *err)
	}
}

func (h *templatesHandler) delete(resp http.ResponseWriter, req *http.Request) {
	id := getTemplateID(req)
	err := h.templates.Delete(id)

	// TODO(wfarner): Convert to use handleError() for status code mapping.
	if err != nil {
		respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown template:%s", id.Name))
		return
	}
}

func getTemplateID(request *http.Request) storage.TemplateID {
	return storage.TemplateID{
		Provisioner: getProvisionerName(request),
		Name:        mux.Vars(request)["key"],
	}
}

func getProvisionerName(request *http.Request) string {
	return mux.Vars(request)["provisioner"]
}

func (h *templatesHandler) attachTo(router *mux.Router) {
	router.HandleFunc("/json", h.getAll).Methods("GET")
	router.HandleFunc("/meta/{provisioner}/json", h.getBlank).Methods("GET")
	router.HandleFunc("/{provisioner}/{key}/create", h.create).Methods("POST")
	router.HandleFunc("/{provisioner}/{key}", h.update).Methods("PUT")
	router.HandleFunc("/{provisioner}/{key}/json", h.getOne).Methods("GET")
	router.HandleFunc("/{provisioner}/{key}", h.delete).Methods("DELETE")
}
