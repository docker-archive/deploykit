package command

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	rest "github.com/conductant/gohm/pkg/server"
	"github.com/docker/libmachete"
	lib "github.com/docker/libmachete/provisioners/api"
	_ "github.com/docker/libmachete/provisioners/aws"
	_ "github.com/docker/libmachete/provisioners/azure"
	"github.com/docker/libmachete/storage/filestores"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

type apiOptions struct {
	Port    int
	RootDir string
}

type api struct {
	options     apiOptions
	credentials libmachete.Credentials
}

func (s *api) init() error {
	store, err := filestores.NewCredentials(s.options.RootDir)
	if err != nil {
		return err
	}
	s.credentials = libmachete.NewCredentials(store)
	return nil
}

func respondError(code int, resp http.ResponseWriter, err error) {
	resp.WriteHeader(code)
	resp.Header().Set("Content-Type", "application/json")
	message := strings.Replace(err.Error(), "\"", "'", -1)
	resp.Write([]byte(fmt.Sprintf("{\"error\":\"%s\"}", message)))
}

func (s *api) start() <-chan error {
	shutdown := make(chan struct{})
	stop, stopped := rest.NewService().DisableAuth().
		ListenPort(s.options.Port).
		Route(
			rest.Endpoint{
				UrlRoute:   "/quitquitquit",
				HttpMethod: rest.POST,
			}).
		To(
			func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
				log.Infoln("Stopping the api....")
				close(shutdown)
			}).
		OnShutdown(
			func() error {
				log.Infoln("Executing user custom shutdown...")
				return nil
			}).
		Route(
			rest.Endpoint{
				UrlRoute:   "/credentials/json",
				HttpMethod: rest.GET,
			}).
		To(
			func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
				log.Infoln("List credentials")
				all, err := s.credentials.ListIds()
				if err != nil {
					respondError(http.StatusInternalServerError, resp, err)
					return
				}
				libmachete.ContentTypeJSON.Respond(resp, all)
			}).
		Route(
			rest.Endpoint{
				UrlRoute:   "/credentials/{key}/create",
				HttpMethod: rest.POST,
				UrlQueries: rest.UrlQueries{
					"provisioner": "",
				},
			}).
		To(
			func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
				provisioner := rest.GetUrlParameter(req, "provisioner")
				key := rest.GetUrlParameter(req, "key")
				log.Infof("Add credential %v, %v\n", provisioner, key)

				err := libmachete.CreateCredential(s.credentials, provisioner, key, req.Body)

				if err == nil {
					return
				}

				switch err.Code {
				case ErrCredentialDuplicate:
					respondError(http.StatusConflict, resp, err)
					return
				case ErrCredentialNotFound:
					respondError(http.StatusNotFound, resp, err)
					return
				default:
					respondError(http.StatusInternalServerError, resp, err)
					return
				}
			}).
		Route(
			rest.Endpoint{
				UrlRoute:   "/credentials/{key}",
				HttpMethod: rest.PUT,
			}).
		To(
			func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
				key := rest.GetUrlParameter(req, "key")
				log.Infof("Update credential %v\n", key)

				buff, err := ioutil.ReadAll(req.Body)
				if err != nil {
					respondError(http.StatusInternalServerError, resp, fmt.Errorf("cannot read input"))
					return
				}

				if !s.credentials.Exists(key) {
					respondError(http.StatusNotFound, resp, fmt.Errorf("Credential does not exist: %v", key))
					return
				}

				base := new(lib.CredentialBase)
				if err = s.credentials.Unmarshal(libmachete.CodecByContentTypeHeader(req), buff, base); err != nil {
					respondError(http.StatusNotFound, resp, fmt.Errorf("Bad input:", string(buff)))
					return
				}

				detail, err := s.credentials.NewCredential(base.ProvisionerName())
				if err != nil {
					respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown provisioner:%s", base.ProvisionerName()))
					return
				}

				if err = s.credentials.Unmarshal(libmachete.CodecByContentTypeHeader(req), buff, detail); err != nil {
					respondError(http.StatusNotFound, resp, fmt.Errorf("Bad input:", string(buff)))
					return
				}

				if err = s.credentials.Save(key, detail); err != nil {
					respondError(http.StatusInternalServerError, resp, err)
					return
				}
			}).
		Route(
			rest.Endpoint{
				UrlRoute:   "/credentials/{key}/json",
				HttpMethod: rest.GET,
			}).
		To(
			func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
				key := rest.GetUrlParameter(req, "key")
				cr, err := s.credentials.Get(key)
				if err != nil {
					respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown credential:%s", key))
					return
				}
				libmachete.ContentTypeJSON.Respond(resp, cr)
			}).
		Route(
			rest.Endpoint{
				UrlRoute:   "/credentials/{key}/yaml",
				HttpMethod: rest.GET,
			}).
		To(
			func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
				key := rest.GetUrlParameter(req, "key")
				cr, err := s.credentials.Get(key)
				if err != nil {
					respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown credential:%s", key))
					return
				}
				libmachete.ContentTypeYAML.Respond(resp, cr)
			}).
		Route(
			rest.Endpoint{
				UrlRoute:   "/credentials/{key}",
				HttpMethod: rest.DELETE,
			}).
		To(
			func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
				key := rest.GetUrlParameter(req, "key")
				err := s.credentials.Delete(key)
				if err != nil {
					respondError(http.StatusNotFound, resp, fmt.Errorf("Unknown credential:%s", key))
					return
				}
			}).
		Start()

	// For stopping the api
	go func() {
		<-shutdown
		close(stop)
	}()
	return stopped
}

func rootDir() string {
	p := os.Getenv("HOME")
	u, err := user.Current()
	if err == nil {
		p = u.HomeDir
	}
	return filepath.Join(p, ".machete")
}

var (
	apiServer = &api{}
)

func ServerCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "start the HTTP server",
		RunE: func(_ *cobra.Command, args []string) error {
			log.Infoln("Starting server:", apiServer)
			err := apiServer.init()
			if err != nil {
				return err
			}
			stopped := apiServer.start()
			<-stopped
			log.Infoln("Bye")
			return nil
		},
	}

	cmd.Flags().IntVar(&apiServer.options.Port, "port", 8888, "Port the server listens on.")
	cmd.Flags().StringVar(&apiServer.options.RootDir, "dir", rootDir(), "Root directory for file storage")
	return cmd
}
