package command

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	rest "github.com/conductant/gohm/pkg/server"
	"github.com/docker/libmachete"
	"golang.org/x/net/context"
	"net/http"
)

type contextHandler struct {
	contexts libmachete.Contexts
}

func (h *contextHandler) getOne(codec *libmachete.Codec) rest.Handler {
	return func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
		key := rest.GetUrlParameter(req, "key")
		cr, err := h.contexts.Get(key)
		if err == nil {
			codec.Respond(resp, cr)
		} else {
			respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown context:%s", key))
		}
	}
}

func (h *contextHandler) getAll(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	log.Infoln("List contexts")
	all, err := h.contexts.ListIds()
	if err != nil {
		respondError(http.StatusInternalServerError, resp, err)
		return
	}
	libmachete.ContentTypeJSON.Respond(resp, all)
}

func (h *contextHandler) create(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	key := rest.GetUrlParameter(req, "key")
	log.Infof("Add context %v", key)

	err := h.contexts.CreateContext(key, req.Body, libmachete.CodecByContentTypeHeader(req))

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

func (h *contextHandler) update(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	key := rest.GetUrlParameter(req, "key")
	log.Infof("Update context %v", key)

	err := h.contexts.UpdateContext(key, req.Body, libmachete.CodecByContentTypeHeader(req))

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

func (h *contextHandler) delete(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	key := rest.GetUrlParameter(req, "key")
	err := h.contexts.Delete(key)
	if err != nil {
		respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown context:%s", key))
		return
	}
}

func contextRoutes(contexts libmachete.Contexts) map[*rest.Endpoint]rest.Handler {
	handler := contextHandler{contexts: contexts}

	return map[*rest.Endpoint]rest.Handler{
		&rest.Endpoint{
			UrlRoute:   "/contexts/json",
			HttpMethod: rest.GET,
		}: handler.getAll,
		&rest.Endpoint{
			UrlRoute:   "/contexts/{key}/create",
			HttpMethod: rest.POST,
		}: handler.create,
		&rest.Endpoint{
			UrlRoute:   "/contexts/{key}",
			HttpMethod: rest.PUT,
		}: handler.update,
		&rest.Endpoint{
			UrlRoute:   "/contexts/{key}/json",
			HttpMethod: rest.GET,
		}: handler.getOne(libmachete.ContentTypeJSON),
		&rest.Endpoint{
			UrlRoute:   "/contexts/{key}/yaml",
			HttpMethod: rest.GET,
		}: handler.getOne(libmachete.ContentTypeYAML),
		&rest.Endpoint{
			UrlRoute:   "/contexts/{key}",
			HttpMethod: rest.DELETE,
		}: handler.delete,
	}
}
