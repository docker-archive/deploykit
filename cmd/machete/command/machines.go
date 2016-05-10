package command

import (
	"bytes"
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
	"io/ioutil"
	"net/http"
)

type machines struct {
	output console.Console
}

func machinesCmd(output console.Console,
	registry *provisioners.Registry,
	machines libmachete.Machines) *cobra.Command {

	cmd := create{
		output:         output,
		machineCreator: libmachete.NewCreator(registry, machines)}

	return &cobra.Command{
		Use:   "create provisioner machine",
		Short: "create a machine",
		RunE: func(_ *cobra.Command, args []string) error {
			return cmd.run(args)
		},
	}
}

func machineRoutes(c libmachete.Credentials, m libmachete.Machines) map[*rest.Endpoint]rest.Handler {
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
			credentials := rest.GetUrlParameter(req, "credentials")
			template := rest.GetUrlParameter(req, "template")
			key := rest.GetUrlParameter(req, "key")

			log.Infof("Add machine %v, %v as %v", template, key, credential)

			buff, err := ioutil.ReadAll(req.Body)
			if err != nil {
				respondError(http.StatusInternalServerError, resp, err)
				return
			}

			// TODO - load up the context
			ctx := context.Background()

			cred, err := c.Get(credentials)
			if err != nil {
				respondError(http.StatusNotFound, resp, err)
				return
			}

			err := t.CreateMachine(ctx, cred, key, bytes.NewBuffer(buff), libmachete.CodecByContentTypeHeader(req))

			if err == nil {
				return
			}

			switch err.Code {
			case libmachete.ErrMachineDuplicate:
				respondError(http.StatusConflict, resp, err)
				return
			case libmachete.ErrMachineNotFound:
				respondError(http.StatusNotFound, resp, err)
				return
			default:
				respondError(http.StatusInternalServerError, resp, err)
				return
			}
		},
		&rest.Endpoint{
			UrlRoute:   "/machines/{provisioner}/{key}",
			HttpMethod: rest.PUT,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			provisioner := rest.GetUrlParameter(req, "provisioner")
			key := rest.GetUrlParameter(req, "key")
			log.Infof("Update machine %v\n", key)

			err := t.UpdateMachine(provisioner, key, req.Body, libmachete.CodecByContentTypeHeader(req))

			if err == nil {
				return
			}

			switch err.Code {
			case libmachete.ErrMachineDuplicate:
				respondError(http.StatusConflict, resp, err)
				return
			case libmachete.ErrMachineNotFound:
				respondError(http.StatusNotFound, resp, err)
				return
			default:
				respondError(http.StatusInternalServerError, resp, err)
				return
			}
		},
		&rest.Endpoint{
			UrlRoute:   "/machines/{provisioner}/{key}/json",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			provisioner := rest.GetUrlParameter(req, "provisioner")
			key := rest.GetUrlParameter(req, "key")
			cr, err := t.Get(provisioner, key)
			if err != nil {
				respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown machine:%s", key))
				return
			}
			libmachete.ContentTypeJSON.Respond(resp, cr)
		},
		&rest.Endpoint{
			UrlRoute:   "/machines/{provisioner}/{key}/yaml",
			HttpMethod: rest.GET,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			provisioner := rest.GetUrlParameter(req, "provisioner")
			key := rest.GetUrlParameter(req, "key")
			cr, err := t.Get(provisioner, key)
			if err != nil {
				respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown machine:%s", key))
				return
			}
			libmachete.ContentTypeYAML.Respond(resp, cr)
		},
		&rest.Endpoint{
			UrlRoute:   "/machines/{provisioner}/{key}",
			HttpMethod: rest.DELETE,
		}: func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			provisioner := rest.GetUrlParameter(req, "provisioner")
			key := rest.GetUrlParameter(req, "key")
			err := t.Delete(provisioner, key)
			if err != nil {
				respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown machine:%s", key))
				return
			}
		},
	}
}
