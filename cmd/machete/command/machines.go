package command

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	rest "github.com/conductant/gohm/pkg/server"
	"github.com/docker/libmachete"
	"golang.org/x/net/context"
	"net/http"
)

func machineRoutes(
	x libmachete.Contexts,
	c libmachete.Credentials,
	t libmachete.Templates,
	m libmachete.Machines) map[*rest.Endpoint]rest.Handler {

	return map[*rest.Endpoint]rest.Handler{
		&rest.Endpoint{
			UrlRoute:   "/machines/json",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			log.Infoln("List machines")
			all, err := m.ListIds()
			if err != nil {
				respondError(http.StatusInternalServerError, resp, err)
				return
			}
			libmachete.ContentTypeJSON.Respond(resp, all)
		},
		&rest.Endpoint{
			UrlRoute:   "/machines/{key}/create",
			HttpMethod: rest.POST,
			UrlQueries: rest.UrlQueries{
				"template":    "default",
				"credentials": "default",
				"context":     "default",
			},
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			context := rest.GetUrlParameter(req, "context")
			credentials := rest.GetUrlParameter(req, "credentials")
			template := rest.GetUrlParameter(req, "template")
			key := rest.GetUrlParameter(req, "key")

			// BUG
			if context == "" {
				context = "default"
			}
			if credentials == "" {
				credentials = "default"
			}
			if template == "" {
				template = "default"
			}

			log.Infof("Add machine context=%v, template=%v, key=%v as %v", context, template, key, credentials)

			// credential
			cred, err := c.Get(credentials)
			if err != nil {
				respondError(http.StatusNotFound, resp, err)
				return
			}

			provisioner := cred.ProvisionerName()

			// Runtime context
			runContext, err := x.Get(context)
			if err != nil {
				respondError(http.StatusNotFound, resp, err)
				return
			}

			// Configure the provisioner
			ctx = libmachete.BuildContext(provisioner, ctx, runContext)

			// Load template
			tpl, err := t.Get(provisioner, template)
			if err != nil {
				respondError(http.StatusNotFound, resp, err)
				return
			}

			events, machineErr := m.CreateMachine(ctx, cred, tpl, key, req.Body, libmachete.CodecByContentTypeHeader(req))

			if machineErr == nil {
				return
			}

			switch machineErr.Code {
			case libmachete.ErrDuplicate:
				respondError(http.StatusConflict, resp, machineErr)
				return
			case libmachete.ErrNotFound:
				respondError(http.StatusNotFound, resp, machineErr)
				return
			default:
				respondError(http.StatusInternalServerError, resp, machineErr)
				return
			}
		},
		&rest.Endpoint{
			UrlRoute:   "/machines/{key}/json",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			key := rest.GetUrlParameter(req, "key")
			mr, err := m.Get(key)
			if err != nil {
				respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown machine:%s", key))
				return
			}
			libmachete.ContentTypeJSON.Respond(resp, mr)
		},
		&rest.Endpoint{
			UrlRoute:   "/machines/{key}/yaml",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			key := rest.GetUrlParameter(req, "key")
			mr, err := m.Get(key)
			if err != nil {
				respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown machine:%s", key))
				return
			}
			libmachete.ContentTypeYAML.Respond(resp, mr)
		},
		&rest.Endpoint{
			UrlRoute:   "/machines/{key}",
			HttpMethod: rest.DELETE,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			key := rest.GetUrlParameter(req, "key")
			err := m.Delete(key)
			if err != nil {
				respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown machine:%s", key))
				return
			}
		},
	}
}
