package command

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	rest "github.com/conductant/gohm/pkg/server"
	"github.com/docker/libmachete"
	"golang.org/x/net/context"
	"net/http"
)

type keyHandler struct {
	keys libmachete.Keys
}

func (h *keyHandler) getOne(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	keyName := rest.GetUrlParameter(req, "key")

	if !h.keys.Exists(keyName) {
		respondError(http.StatusNotFound, resp, fmt.Errorf("key not exists:%v", keyName))
		return
	}

	publicKey, err := h.keys.GetPublicKey(keyName)
	if err != nil {
		respondError(http.StatusInternalServerError, resp, err)
		return
	}

	resp.Header().Set("Content-Type", "text/plain")
	resp.Write(publicKey)
}

func (h *keyHandler) getAll(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	log.Infoln("List keys")
	all, err := h.keys.ListIds()
	if err != nil {
		respondError(http.StatusInternalServerError, resp, err)
		return
	}
	libmachete.ContentTypeJSON.Respond(resp, all)
}

func (h *keyHandler) create(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	keyName := rest.GetUrlParameter(req, "key")

	log.Infof("Add key %v", keyName)

	if h.keys.Exists(keyName) {
		respondError(http.StatusConflict, resp, fmt.Errorf("key exists:%v", keyName))
		return
	}

	err := h.keys.NewKeyPair(keyName)
	if err != nil {
		respondError(http.StatusInternalServerError, resp, err)
		return
	}
	return
}

func (h *keyHandler) delete(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	keyName := rest.GetUrlParameter(req, "key")
	log.Infof("Delete key %v", keyName)

	if !h.keys.Exists(keyName) {
		respondError(http.StatusNotFound, resp, fmt.Errorf("key not exists:%v", keyName))
		return
	}

	err := h.keys.Remove(keyName)
	if err != nil {
		respondError(http.StatusInternalServerError, resp, err)
		return
	}
	return
}

func keyRoutes(keys libmachete.Keys) map[*rest.Endpoint]rest.Handler {

	handler := keyHandler{
		keys: keys,
	}

	return map[*rest.Endpoint]rest.Handler{
		&rest.Endpoint{
			UrlRoute:   "/keys/json",
			HttpMethod: rest.GET,
		}: handler.getAll,
		&rest.Endpoint{
			UrlRoute:   "/keys/{key}/create",
			HttpMethod: rest.POST,
		}: handler.create,
		&rest.Endpoint{
			UrlRoute:   "/keys/{key}",
			HttpMethod: rest.GET,
		}: handler.getOne,
		&rest.Endpoint{
			UrlRoute:   "/keys/{key}",
			HttpMethod: rest.DELETE,
		}: handler.delete,
	}
}
