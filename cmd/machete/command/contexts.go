package command

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	rest "github.com/conductant/gohm/pkg/server"
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/cmd/machete/console"
	"github.com/docker/libmachete/provisioners"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"net/http"
)

type contexts struct {
	output console.Console
}

func contextsCmd(output console.Console,
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

func contextRoutes(c libmachete.Contexts) map[*rest.Endpoint]rest.Handler {
	return map[*rest.Endpoint]rest.Handler{
		&rest.Endpoint{
			UrlRoute:   "/contexts/json",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			log.Infoln("List contexts")
			all, err := c.ListIds()
			if err != nil {
				respondError(http.StatusInternalServerError, resp, err)
				return
			}
			libmachete.ContentTypeJSON.Respond(resp, all)
		},
		&rest.Endpoint{
			UrlRoute:   "/contexts/{key}/create",
			HttpMethod: rest.POST,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			key := rest.GetUrlParameter(req, "key")
			log.Infof("Add context %v", key)

			err := c.CreateContext(key, req.Body, libmachete.CodecByContentTypeHeader(req))

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
			UrlRoute:   "/contexts/{key}",
			HttpMethod: rest.PUT,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			key := rest.GetUrlParameter(req, "key")
			log.Infof("Update context %v", key)

			err := c.UpdateContext(key, req.Body, libmachete.CodecByContentTypeHeader(req))

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
			UrlRoute:   "/contexts/{key}/json",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			key := rest.GetUrlParameter(req, "key")
			cr, err := c.Get(key)
			if err != nil {
				respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown context:%s", key))
				return
			}
			libmachete.ContentTypeJSON.Respond(resp, cr)
		},
		&rest.Endpoint{
			UrlRoute:   "/contexts/{key}/yaml",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			key := rest.GetUrlParameter(req, "key")
			cr, err := c.Get(key)
			if err != nil {
				respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown context:%s", key))
				return
			}
			libmachete.ContentTypeYAML.Respond(resp, cr)
		},
		&rest.Endpoint{
			UrlRoute:   "/contexts/{key}",
			HttpMethod: rest.DELETE,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			key := rest.GetUrlParameter(req, "key")
			err := c.Delete(key)
			if err != nil {
				respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown context:%s", key))
				return
			}
		},
	}
}
