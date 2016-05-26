package http

import (
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/storage"
	"github.com/gorilla/mux"
	"net/http"
)

type templatesHandler struct {
	templates libmachete.Templates
}

func (h *templatesHandler) getOne(req *http.Request) (interface{}, *libmachete.Error) {
	return h.templates.Get(getTemplateID(req))
}

func (h *templatesHandler) getBlank(req *http.Request) (interface{}, *libmachete.Error) {
	return h.templates.NewBlankTemplate(getProvisionerName(req))
}

func (h *templatesHandler) getAll(req *http.Request) (interface{}, *libmachete.Error) {
	return h.templates.ListIds()
}

func (h *templatesHandler) create(req *http.Request) (interface{}, *libmachete.Error) {
	return nil, h.templates.CreateTemplate(getTemplateID(req), req.Body, libmachete.ContentTypeJSON)
}

func (h *templatesHandler) update(req *http.Request) (interface{}, *libmachete.Error) {
	return nil, h.templates.UpdateTemplate(getTemplateID(req), req.Body, libmachete.ContentTypeJSON)
}

func (h *templatesHandler) delete(req *http.Request) (interface{}, *libmachete.Error) {
	return nil, h.templates.Delete(getTemplateID(req))
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
	router.HandleFunc("/json", outputHandler(h.getAll)).Methods("GET")
	router.HandleFunc("/meta/{provisioner}/json", outputHandler(h.getBlank)).Methods("GET")
	router.HandleFunc("/{provisioner}/{key}/create", outputHandler(h.create)).Methods("POST")
	router.HandleFunc("/{provisioner}/{key}", outputHandler(h.update)).Methods("PUT")
	router.HandleFunc("/{provisioner}/{key}/json", outputHandler(h.getOne)).Methods("GET")
	router.HandleFunc("/{provisioner}/{key}", outputHandler(h.delete)).Methods("DELETE")
}
