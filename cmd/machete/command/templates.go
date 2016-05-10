package command

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	rest "github.com/conductant/gohm/pkg/server"
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/cmd/machete/console"
	"github.com/docker/libmachete/provisioners"
	_ "github.com/docker/libmachete/provisioners/aws"
	_ "github.com/docker/libmachete/provisioners/azure"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"net/http"
)

type templates struct {
	output console.Console
}

func templatesCmd(output console.Console,
	registry *provisioners.Registry,
	templates libmachete.Templates) *cobra.Command {

	cmd := create{
		output:         output,
		machineCreator: libmachete.NewCreator(registry, templates)}

	return &cobra.Command{
		Use:   "create provisioner template",
		Short: "create a machine",
		RunE: func(_ *cobra.Command, args []string) error {
			return cmd.run(args)
		},
	}
}

func templateRoutes(t libmachete.Templates) map[*rest.Endpoint]rest.Handler {
	return map[*rest.Endpoint]rest.Handler{
		&rest.Endpoint{
			UrlRoute:   "/templates/json",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			log.Infoln("List templates")
			all, err := t.ListIds()
			if err != nil {
				respondError(http.StatusInternalServerError, resp, err)
				return
			}
			libmachete.ContentTypeJSON.Respond(resp, all)
		},
		&rest.Endpoint{
			UrlRoute:   "/meta/{provisioner}/json",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			provisioner := rest.GetUrlParameter(req, "provisioner")
			log.Infof("Get template example %v", provisioner)

			example, err := t.NewTemplate(provisioner)
			if err != nil {
				respondError(http.StatusNotFound, resp, err)
				return
			}
			libmachete.ContentTypeJSON.Respond(resp, example)
		},
		&rest.Endpoint{
			UrlRoute:   "/meta/{provisioner}/yaml",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			provisioner := rest.GetUrlParameter(req, "provisioner")
			log.Infof("Get template example %v", provisioner)

			example, err := t.NewTemplate(provisioner)
			if err != nil {
				respondError(http.StatusNotFound, resp, err)
				return
			}
			libmachete.ContentTypeYAML.Respond(resp, example)
		},
		&rest.Endpoint{
			UrlRoute:   "/templates/{provisioner}/{key}/create",
			HttpMethod: rest.POST,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			provisioner := rest.GetUrlParameter(req, "provisioner")
			key := rest.GetUrlParameter(req, "key")
			log.Infof("Add template %v, %v\n", provisioner, key)

			err := t.CreateTemplate(provisioner, key, req.Body, libmachete.CodecByContentTypeHeader(req))

			if err == nil {
				return
			}

			switch err.Code {
			case libmachete.ErrTemplateDuplicate:
				respondError(http.StatusConflict, resp, err)
				return
			case libmachete.ErrTemplateNotFound:
				respondError(http.StatusNotFound, resp, err)
				return
			default:
				respondError(http.StatusInternalServerError, resp, err)
				return
			}
		},
		&rest.Endpoint{
			UrlRoute:   "/templates/{provisioner}/{key}",
			HttpMethod: rest.PUT,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			provisioner := rest.GetUrlParameter(req, "provisioner")
			key := rest.GetUrlParameter(req, "key")
			log.Infof("Update template %v\n", key)

			err := t.UpdateTemplate(provisioner, key, req.Body, libmachete.CodecByContentTypeHeader(req))

			if err == nil {
				return
			}

			switch err.Code {
			case libmachete.ErrTemplateDuplicate:
				respondError(http.StatusConflict, resp, err)
				return
			case libmachete.ErrTemplateNotFound:
				respondError(http.StatusNotFound, resp, err)
				return
			default:
				respondError(http.StatusInternalServerError, resp, err)
				return
			}
		},
		&rest.Endpoint{
			UrlRoute:   "/templates/{provisioner}/{key}/json",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			provisioner := rest.GetUrlParameter(req, "provisioner")
			key := rest.GetUrlParameter(req, "key")
			cr, err := t.Get(provisioner, key)
			if err != nil {
				respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown template:%s", key))
				return
			}
			libmachete.ContentTypeJSON.Respond(resp, cr)
		},
		&rest.Endpoint{
			UrlRoute:   "/templates/{provisioner}/{key}/yaml",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			provisioner := rest.GetUrlParameter(req, "provisioner")
			key := rest.GetUrlParameter(req, "key")
			cr, err := t.Get(provisioner, key)
			if err != nil {
				respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown template:%s", key))
				return
			}
			libmachete.ContentTypeYAML.Respond(resp, cr)
		},
		&rest.Endpoint{
			UrlRoute:   "/templates/{provisioner}/{key}",
			HttpMethod: rest.DELETE,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			provisioner := rest.GetUrlParameter(req, "provisioner")
			key := rest.GetUrlParameter(req, "key")
			err := t.Delete(provisioner, key)
			if err != nil {
				respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown template:%s", key))
				return
			}
		},
	}
}
