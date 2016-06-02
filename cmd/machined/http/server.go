package http

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/storage/filestores"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
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
	keystore    libmachete.Keys
}

func build(sandbox filestores.Sandbox, provisioners *libmachete.MachineProvisioners) (*apiServer, error) {
	credentialsSandbox := sandbox.Nested("credentials")
	templatesSandbox := sandbox.Nested("templates")
	machinesSandbox := sandbox.Nested("machines")
	for _, box := range []filestores.Sandbox{credentialsSandbox, templatesSandbox, machinesSandbox} {
		err := box.Mkdirs()
		if err != nil {
			return nil, err
		}
	}

	return &apiServer{
		credentials: libmachete.NewCredentials(filestores.NewCredentials(credentialsSandbox), provisioners),
		templates:   libmachete.NewTemplates(filestores.NewTemplates(templatesSandbox), provisioners),
		machines:    libmachete.NewMachines(filestores.NewMachines(machinesSandbox)),
		keystore:    libmachete.NewKeys(filestores.NewKeys(machinesSandbox)), // Keys are stored with host/instance data
	}, nil
}

// TODO(wfarner): Consider collapsing this or moving it closer to outputHandler.
func respondError(code int, resp http.ResponseWriter, err error) {
	resp.WriteHeader(code)
	resp.Header().Set("Content-Type", "application/json")
	body, err := json.Marshal(map[string]string{"error": err.Error()})
	if err == nil {
		resp.Write(body)
		return
	}
	panic(fmt.Sprintf("Failed to marshal error text: %s", err.Error()))
}

type routerAttachment interface {
	attachTo(router *mux.Router)
}

func (s *apiServer) getHandler() http.Handler {
	attachments := map[string]routerAttachment{
		"/credentials": &credentialsHandler{credentials: s.credentials},
		"/templates":   &templatesHandler{templates: s.templates},
		"/machines": &machineHandler{
			creds:     s.credentials,
			templates: s.templates,
			machines:  s.machines,
			keystore:  s.keystore},
	}

	router := mux.NewRouter()

	for path, attachment := range attachments {
		attachment.attachTo(router.PathPrefix(path).Subrouter())
	}

	return router
}

func (s *apiServer) start() error {
	handler := s.getHandler()
	http.Handle("/", handler)
	return http.ListenAndServe(fmt.Sprintf(":%s", s.options.Port), handler)
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

			server, err := build(filestores.NewOsSandbox(options.RootDir), &libmachete.DefaultProvisioners)
			if err != nil {
				return err
			}
			err = server.start()
			log.Error(err)
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
