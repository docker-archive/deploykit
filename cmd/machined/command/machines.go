package command

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	rest "github.com/conductant/gohm/pkg/server"
	"github.com/docker/libmachete"
	"golang.org/x/net/context"
	"net/http"
)

type machineHandler struct {
	provisionerContexts libmachete.Contexts
	creds               libmachete.Credentials
	templates           libmachete.Templates
	machines            libmachete.Machines
}

func (h *machineHandler) getOne(codec *libmachete.Codec) rest.Handler {
	return func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
		key := rest.GetUrlParameter(req, "key")
		mr, err := h.machines.Get(key)
		if err == nil {
			codec.Respond(resp, mr)
		} else {
			respondError(
				http.StatusNotFound,
				resp,
				fmt.Errorf("Unknown machine:%s, err=%s", key, err.Error()))
		}
	}
}

func (h *machineHandler) getAll(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	log.Infoln("List machines")
	long := len(rest.GetUrlParameter(req, "long")) > 0

	if long {
		all, err := h.machines.List()
		if err != nil {
			respondError(http.StatusInternalServerError, resp, err)
			return
		}
		libmachete.ContentTypeJSON.Respond(resp, all)
		return
	}

	all, err := h.machines.ListIds()
	if err != nil {
		respondError(http.StatusInternalServerError, resp, err)
		return
	}
	libmachete.ContentTypeJSON.Respond(resp, all)
}

func (h *machineHandler) create(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	context := rest.GetUrlParameter(req, "context")
	credentials := rest.GetUrlParameter(req, "credentials")
	template := rest.GetUrlParameter(req, "template")

	// TODO - fix this in framework to return default values
	if context == "" {
		context = "default"
	}
	if credentials == "" {
		credentials = "default"
	}
	if template == "" {
		template = "default"
	}

	log.Infof("Add machine context=%v, template=%v, as %v", context, template, credentials)

	// credential
	cred, err := h.creds.Get(credentials)
	if err != nil {
		respondError(http.StatusNotFound, resp, err)
		return
	}

	provisionerName := cred.ProvisionerName()

	// Runtime context
	runContext, err := h.provisionerContexts.Get(context)
	if err != nil {
		respondError(http.StatusNotFound, resp, err)
		return
	}

	// Configure the provisioner context
	ctx = libmachete.BuildContext(provisionerName, ctx, runContext)

	// Get the provisioner
	// TODO - clean up the error code to better reflect the nature of error
	provisioner, err := libmachete.GetProvisioner(provisionerName, ctx, cred)
	if err != nil {
		respondError(http.StatusNotFound, resp, err)
		return
	}

	// Load template
	tpl, err := h.templates.Get(provisionerName, template)
	if err != nil {
		respondError(http.StatusNotFound, resp, err)
		return
	}

	events, machineErr := h.machines.CreateMachine(provisioner, ctx, cred, tpl,
		req.Body, libmachete.CodecByContentTypeHeader(req))

	if machineErr == nil {
		// TODO - if the client requests streaming events... send that out here.
		go func() {
			for event := range events {
				log.Infoln("Event:", event)
			}
		}()
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
}

func machineRoutes(
	provisionerContexts libmachete.Contexts,
	creds libmachete.Credentials,
	templates libmachete.Templates,
	machines libmachete.Machines) map[*rest.Endpoint]rest.Handler {

	handler := machineHandler{
		provisionerContexts: provisionerContexts,
		creds:               creds,
		templates:           templates,
		machines:            machines,
	}

	return map[*rest.Endpoint]rest.Handler{
		&rest.Endpoint{
			UrlRoute:   "/machines/json",
			HttpMethod: rest.GET,
			UrlQueries: rest.UrlQueries{
				"long": false,
			},
		}: handler.getAll,
		&rest.Endpoint{
			UrlRoute:   "/machines/create",
			HttpMethod: rest.POST,
			UrlQueries: rest.UrlQueries{
				"template":    "default",
				"credentials": "default",
				"context":     "default",
			},
		}: handler.create,
		&rest.Endpoint{
			UrlRoute:   "/machines/{key}/json",
			HttpMethod: rest.GET,
		}: handler.getOne(libmachete.ContentTypeJSON),
		&rest.Endpoint{
			UrlRoute:   "/machines/{key}/yaml",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			key := rest.GetUrlParameter(req, "key")
			mr, err := machines.Get(key)
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
			err := machines.Delete(key)
			if err != nil {
				respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown machine:%s", key))
				return
			}
		},
	}
}
