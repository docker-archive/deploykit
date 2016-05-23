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

func mkdir(parent, child string) (string, error) {
	p := filepath.Join(parent, child)
	return p, os.MkdirAll(p, 0755)
}

func (s *apiServer) init() error {
	provisioners := libmachete.DefaultProvisioners

	credentialsPath, err := mkdir(s.options.RootDir, "credentials")
	if err != nil {
		return err
	}
	templatesPath, err := mkdir(s.options.RootDir, "templates")
	if err != nil {
		return err
	}
	machinesPath, err := mkdir(s.options.RootDir, "machines")
	if err != nil {
		return err
	}

	credentialsStore, err := filestores.NewCredentials(credentialsPath)
	if err != nil {
		return err
	}
	s.credentials = libmachete.NewCredentials(credentialsStore, &provisioners)

	templateStore, err := filestores.NewTemplates(templatesPath)
	if err != nil {
		return err
	}
	s.templates = libmachete.NewTemplates(templateStore, &provisioners)

	machineStore, err := filestores.NewMachines(machinesPath)
	if err != nil {
		return err
	}
	s.machines = libmachete.NewMachines(machineStore)
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

var (
	server = &apiServer{}
)

// ServerCmd returns the serve subcommand.
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

	cmd.Flags().StringVar(
		&server.options.Port,
		"port",
		"8888",
		"Port the server listens on. File path for unix socket")
	cmd.Flags().StringVar(
		&server.options.RootDir,
		"dir",
		rootDir(),
		"Root directory for file storage")
	return cmd
}
