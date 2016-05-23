package http

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	rest "github.com/conductant/gohm/pkg/server"
	"github.com/docker/libmachete"
	"golang.org/x/net/context"
	"net/http"
)

type credentialsHandler struct {
	credentials libmachete.Credentials
}

func (h *credentialsHandler) getOne(codec *libmachete.Codec) rest.Handler {
	return func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
		key := rest.GetUrlParameter(req, "key")
		cr, err := h.credentials.Get(key)
		if err == nil {
			codec.Respond(resp, cr)
		} else {
			respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown credential:%s", key))
			return
		}
	}
}

func (h *credentialsHandler) getAll(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	log.Infoln("List credentials")
	all, err := h.credentials.ListIds()
	if err != nil {
		respondError(http.StatusInternalServerError, resp, err)
		return
	}
	libmachete.ContentTypeJSON.Respond(resp, all)
}

func (h *credentialsHandler) create(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	provisioner := rest.GetUrlParameter(req, "provisioner")
	key := rest.GetUrlParameter(req, "key")
	log.Infof("Add credential %v, %v\n", provisioner, key)

	err := h.credentials.CreateCredential(provisioner, key, req.Body, libmachete.CodecByContentTypeHeader(req))

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

func (h *credentialsHandler) update(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	key := rest.GetUrlParameter(req, "key")
	log.Infof("Update credential %v\n", key)

	err := h.credentials.UpdateCredential(key, req.Body, libmachete.CodecByContentTypeHeader(req))

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

func (h *credentialsHandler) delete(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	key := rest.GetUrlParameter(req, "key")
	err := h.credentials.Delete(key)
	if err != nil {
		respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown credential:%s", key))
		return
	}
}

func credentialRoutes(credentials libmachete.Credentials) map[*rest.Endpoint]rest.Handler {
	handler := credentialsHandler{credentials: credentials}

	return map[*rest.Endpoint]rest.Handler{
		&rest.Endpoint{
			UrlRoute:   "/credentials/json",
			HttpMethod: rest.GET,
		}: handler.getAll,
		&rest.Endpoint{
			UrlRoute:   "/credentials/{key}/create",
			HttpMethod: rest.POST,
			UrlQueries: rest.UrlQueries{
				"provisioner": "",
			},
		}: handler.create,
		&rest.Endpoint{
			UrlRoute:   "/credentials/{key}",
			HttpMethod: rest.PUT,
		}: handler.update,
		&rest.Endpoint{
			UrlRoute:   "/credentials/{key}/json",
			HttpMethod: rest.GET,
		}: handler.getOne(libmachete.ContentTypeJSON),
		&rest.Endpoint{
			UrlRoute:   "/credentials/{key}/yaml",
			HttpMethod: rest.GET,
		}: handler.getOne(libmachete.ContentTypeYAML),
		&rest.Endpoint{
			UrlRoute:   "/credentials/{key}",
			HttpMethod: rest.DELETE,
		}: handler.delete,
	}
}
