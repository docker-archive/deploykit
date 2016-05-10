package command

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	rest "github.com/conductant/gohm/pkg/server"
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/storage/filestores"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
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

type apiServer struct {
	options     apiOptions
	credentials libmachete.Credentials
	templates   libmachete.Templates
}

func (s *apiServer) init() error {
	cStore, err := filestores.NewCredentials(s.options.RootDir)
	if err != nil {
		return err
	}
	s.credentials = libmachete.NewCredentials(cStore)

	tStore, err := filestores.NewTemplates(s.options.RootDir)
	if err != nil {
		return err
	}
	s.templates = libmachete.NewTemplates(tStore)
	return nil
}

func respondError(code int, resp http.ResponseWriter, err error) {
	resp.WriteHeader(code)
	resp.Header().Set("Content-Type", "application/json")
	message := strings.Replace(err.Error(), "\"", "'", -1)
	resp.Write([]byte(fmt.Sprintf("{\"error\":\"%s\"}", message)))
}

func (s *apiServer) start() <-chan error {
	shutdown := make(chan struct{})
	service := rest.NewService().DisableAuth().
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
			})
	for endpoint, fn := range credentialRoutes(s.credentials) {
		service.Route(*endpoint).To(fn)
	}
	for endpoint, fn := range templateRoutes(s.templates) {
		service.Route(*endpoint).To(fn)
	}

	stop, stopped := service.Start()

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
	server = &apiServer{}
)

func ServerCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "start the HTTP server",
		RunE: func(_ *cobra.Command, args []string) error {
			log.Infoln("Starting server:", server)
			err := server.init()
			if err != nil {
				return err
			}
			stopped := server.start()
			<-stopped
			log.Infoln("Bye")
			return nil
		},
	}

	cmd.Flags().IntVar(&server.options.Port, "port", 8888, "Port the server listens on.")
	cmd.Flags().StringVar(&server.options.RootDir, "dir", rootDir(), "Root directory for file storage")
	return cmd
}
