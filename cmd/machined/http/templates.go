package http

import (
	"github.com/docker/libmachete/api"
	"github.com/gorilla/mux"
	"net/http"
)

type templatesHandler struct {
	templates api.Templates
}

func (h *templatesHandler) getOne(req *http.Request) (interface{}, *api.Error) {
	return h.templates.Get(getTemplateID(req))
}

func (h *templatesHandler) getBlank(req *http.Request) (interface{}, *api.Error) {
	return h.templates.NewBlankTemplate(getProvisionerName(req))
}

func (h *templatesHandler) getAll(req *http.Request) (interface{}, *api.Error) {
	return h.templates.ListIds()
}

func (h *templatesHandler) create(req *http.Request) (interface{}, *api.Error) {
	return nil, h.templates.CreateTemplate(getTemplateID(req), req.Body, api.ContentTypeJSON)
}

func (h *templatesHandler) update(req *http.Request) (interface{}, *api.Error) {
	return nil, h.templates.UpdateTemplate(getTemplateID(req), req.Body, api.ContentTypeJSON)
}

func (h *templatesHandler) delete(req *http.Request) (interface{}, *api.Error) {
	return nil, h.templates.Delete(getTemplateID(req))
}

func getTemplateID(request *http.Request) api.TemplateID {
	return api.TemplateID{
		Provisioner: getProvisionerName(request),
		Name:        mux.Vars(request)["key"],
	}
}

func getProvisionerName(request *http.Request) string {
	return mux.Vars(request)["provisioner"]
}

func (h *templatesHandler) attachTo(router *mux.Router) {
	router.HandleFunc("/", outputHandler(h.getAll)).Methods("GET")
	router.HandleFunc("/meta/{provisioner}", outputHandler(h.getBlank)).Methods("GET")
	router.HandleFunc("/{provisioner}/{key}", outputHandler(h.create)).Methods("POST")
	router.HandleFunc("/{provisioner}/{key}", outputHandler(h.update)).Methods("PUT")
	router.HandleFunc("/{provisioner}/{key}", outputHandler(h.getOne)).Methods("GET")
	router.HandleFunc("/{provisioner}/{key}", outputHandler(h.delete)).Methods("DELETE")
}
