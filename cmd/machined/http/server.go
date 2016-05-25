package http

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

import (
	// Load the supported provisioners
	_ "github.com/docker/libmachete/provisioners/aws"
	_ "github.com/docker/libmachete/provisioners/azure"
)

type apiOptions struct {
	Port    string
	RootDir string
}

type apiServer struct {
	options     apiOptions
	credentials libmachete.Credentials
	templates   libmachete.Templates
	machines    libmachete.Machines
}

func build(options apiOptions) (*apiServer, error) {
	provisioners := libmachete.DefaultProvisioners

	sandbox := filestores.NewOsSandbox(options.RootDir)
	contextSandbox := sandbox.Nested("contexts")
	credentialsSandbox := sandbox.Nested("credentials")
	templatesSandbox := sandbox.Nested("templates")
	machinesSandbox := sandbox.Nested("machines")
	for _, box := range []filestores.Sandbox{contextSandbox, credentialsSandbox, templatesSandbox, machinesSandbox} {
		err := box.Mkdirs()
		if err != nil {
			return nil, err
		}
	}

	return &apiServer{
		credentials: libmachete.NewCredentials(filestores.NewCredentials(credentialsSandbox), &provisioners),
		templates:   libmachete.NewTemplates(filestores.NewTemplates(templatesSandbox), &provisioners),
		machines:    libmachete.NewMachines(filestores.NewMachines(machinesSandbox)),
	}, nil
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
	for endpoint, fn := range machineRoutes(s.credentials, s.templates, s.machines) {
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

// ServerCmd returns the serve subcommand.
func ServerCmd() *cobra.Command {
	options := apiOptions{}

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "start the HTTP server",
		RunE: func(_ *cobra.Command, args []string) error {
			log.Infoln("Starting server")

			server, err := build(options)
			if err != nil {
				return err
			}
			stopped := server.start()
			<-stopped
			log.Infoln("Bye")
			return nil
		},
	}

	cmd.Flags().StringVar(
		&options.Port,
		"port",
		"8888",
		"Port the server listens on. File path for unix socket")
	cmd.Flags().StringVar(
		&options.RootDir,
		"dir",
		rootDir(),
		"Root directory for file storage")
	return cmd
}
