package command

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	rest "github.com/conductant/gohm/pkg/server"
	"github.com/docker/libmachete"
	"golang.org/x/net/context"
	"net/http"
)

func credentialRoutes(c libmachete.Credentials) map[*rest.Endpoint]rest.Handler {
	return map[*rest.Endpoint]rest.Handler{
		&rest.Endpoint{
			UrlRoute:   "/credentials/json",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			log.Infoln("List credentials")
			all, err := c.ListIds()
			if err != nil {
				respondError(http.StatusInternalServerError, resp, err)
				return
			}
			libmachete.ContentTypeJSON.Respond(resp, all)
		},
		&rest.Endpoint{
			UrlRoute:   "/credentials/{key}/create",
			HttpMethod: rest.POST,
			UrlQueries: rest.UrlQueries{
				"provisioner": "",
			},
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			provisioner := rest.GetUrlParameter(req, "provisioner")
			key := rest.GetUrlParameter(req, "key")
			log.Infof("Add credential %v, %v\n", provisioner, key)

			err := c.CreateCredential(provisioner, key, req.Body, libmachete.CodecByContentTypeHeader(req))

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
		},
		&rest.Endpoint{
			UrlRoute:   "/credentials/{key}",
			HttpMethod: rest.PUT,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			key := rest.GetUrlParameter(req, "key")
			log.Infof("Update credential %v\n", key)

			err := c.UpdateCredential(key, req.Body, libmachete.CodecByContentTypeHeader(req))

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
		},
		&rest.Endpoint{
			UrlRoute:   "/credentials/{key}/json",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			key := rest.GetUrlParameter(req, "key")
			cr, err := c.Get(key)
			if err != nil {
				respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown credential:%s", key))
				return
			}
			libmachete.ContentTypeJSON.Respond(resp, cr)
		},
		&rest.Endpoint{
			UrlRoute:   "/credentials/{key}/yaml",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			key := rest.GetUrlParameter(req, "key")
			cr, err := c.Get(key)
			if err != nil {
				respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown credential:%s", key))
				return
			}
			libmachete.ContentTypeYAML.Respond(resp, cr)
		},
		&rest.Endpoint{
			UrlRoute:   "/credentials/{key}",
			HttpMethod: rest.DELETE,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			key := rest.GetUrlParameter(req, "key")
			err := c.Delete(key)
			if err != nil {
				respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown credential:%s", key))
				return
			}
		},
	}
}
