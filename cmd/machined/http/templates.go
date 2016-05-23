package http

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	rest "github.com/conductant/gohm/pkg/server"
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/storage"
	"golang.org/x/net/context"
	"net/http"
)

type templatesHandler struct {
	templates libmachete.Templates
}

func (h *templatesHandler) getOne(codec *libmachete.Codec) rest.Handler {
	return func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
		id := getTemplateID(req)
		cr, err := h.templates.Get(id)
		if err != nil {
			respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown template:%s", id.Name))
			return
		}
		codec.Respond(resp, cr)
	}
}

func (h *templatesHandler) getBlank(codec *libmachete.Codec) rest.Handler {
	return func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
		provisioner := rest.GetUrlParameter(req, "provisioner")
		log.Infof("Get template example %v", provisioner)

		example, err := h.templates.NewBlankTemplate(provisioner)
		if err != nil {
			respondError(http.StatusNotFound, resp, err)
			return
		}
		codec.Respond(resp, example)
	}
}

func (h *templatesHandler) getAll(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	log.Infoln("List templates")
	all, err := h.templates.ListIds()
	if err != nil {
		respondError(http.StatusInternalServerError, resp, err)
		return
	}
	libmachete.ContentTypeJSON.Respond(resp, all)
}

func (h *templatesHandler) create(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
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

func (h *templatesHandler) update(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
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

func (h *templatesHandler) delete(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	id := getTemplateID(req)
	err := h.templates.Delete(id)
	if err != nil {
		respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown template:%s", id.Name))
		return
	}
}

func getTemplateID(request *http.Request) storage.TemplateID {
	return storage.TemplateID{
		Provisioner: rest.GetUrlParameter(request, "provisioner"),
		Name:        rest.GetUrlParameter(request, "key"),
	}
}

func templateRoutes(templates libmachete.Templates) map[*rest.Endpoint]rest.Handler {
	handler := templatesHandler{templates: templates}

	return map[*rest.Endpoint]rest.Handler{
		&rest.Endpoint{
			UrlRoute:   "/templates/json",
			HttpMethod: rest.GET,
		}: handler.getAll,
		&rest.Endpoint{
			UrlRoute:   "/meta/{provisioner}/json",
			HttpMethod: rest.GET,
		}: handler.getBlank(libmachete.ContentTypeJSON),
		&rest.Endpoint{
			UrlRoute:   "/meta/{provisioner}/yaml",
			HttpMethod: rest.GET,
		}: handler.getBlank(libmachete.ContentTypeYAML),
		&rest.Endpoint{
			UrlRoute:   "/templates/{provisioner}/{key}/create",
			HttpMethod: rest.POST,
		}: handler.create,
		&rest.Endpoint{
			UrlRoute:   "/templates/{provisioner}/{key}",
			HttpMethod: rest.PUT,
		}: handler.update,
		&rest.Endpoint{
			UrlRoute:   "/templates/{provisioner}/{key}/json",
			HttpMethod: rest.GET,
		}: handler.getOne(libmachete.ContentTypeJSON),
		&rest.Endpoint{
			UrlRoute:   "/templates/{provisioner}/{key}/yaml",
			HttpMethod: rest.GET,
		}: handler.getOne(libmachete.ContentTypeYAML),
		&rest.Endpoint{
			UrlRoute:   "/templates/{provisioner}/{key}",
			HttpMethod: rest.DELETE,
		}: handler.delete,
	}
}
