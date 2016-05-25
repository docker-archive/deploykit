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

func (h *templatesHandler) getOne(codec *libmachete.Codec) Handler {
	return func(resp http.ResponseWriter, req *http.Request) {
		id := getTemplateID(req)
		cr, err := h.templates.Get(id)
		if err != nil {
			respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown template:%s", id.Name))
			return
		}
		codec.Respond(resp, cr)
	}
}

func (h *templatesHandler) getBlank(codec *libmachete.Codec) Handler {
	return func(resp http.ResponseWriter, req *http.Request) {
		provisioner := getProvisionerName(req)
		log.Infof("Get template example %v", provisioner)

		example, err := h.templates.NewBlankTemplate(provisioner)
		if err != nil {
			respondError(http.StatusNotFound, resp, err)
			return
		}
		codec.Respond(resp, example)
	}
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

func (h *templatesHandler) create(resp http.ResponseWriter, req *http.Request) {
	id := getTemplateID(req)
	log.Infof("Add template %v", id)

	err := h.templates.CreateTemplate(id, req.Body, libmachete.CodecByContentTypeHeader(req))

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

func (h *templatesHandler) update(resp http.ResponseWriter, req *http.Request) {
	id := getTemplateID(req)
	log.Infof("Update template %v", id.Name)

	err := h.templates.UpdateTemplate(id, req.Body, libmachete.CodecByContentTypeHeader(req))

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

func (h *templatesHandler) delete(resp http.ResponseWriter, req *http.Request) {
	id := getTemplateID(req)
	err := h.templates.Delete(id)
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
	return request.URL.Query().Get("provisioner")
}

func (h *templatesHandler) attachTo(router *mux.Router) {
	router.HandleFunc("/json", h.getAll).Methods("GET")
	router.HandleFunc("/meta/{provisioner}/json", h.getBlank(libmachete.ContentTypeJSON)).Methods("GET")
	router.HandleFunc("/meta/{provisioner}/yaml", h.getBlank(libmachete.ContentTypeYAML)).Methods("GET")
	router.HandleFunc("/{provisioner}/{key}/create", h.create).Methods("POST")
	router.HandleFunc("/{provisioner}/{key}", h.update).Methods("PUT")
	router.HandleFunc("/{provisioner}/{key}/json", h.getOne(libmachete.ContentTypeJSON)).Methods("GET")
	router.HandleFunc("/{provisioner}/{key}/yaml", h.getOne(libmachete.ContentTypeYAML)).Methods("GET")
	router.HandleFunc("/{provisioner}/{key}", h.delete).Methods("DELETE")
}
