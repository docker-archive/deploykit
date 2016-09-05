package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/docker/libmachete/controller/hello"
	"github.com/spf13/cobra"
	"time"
)

const (
	// Default host value borrowed from github.com/docker/docker/opts
	defaultDockerEngineAddress = "unix:///var/run/docker.sock"
)

var (
	tlsOptions          = tlsconfig.Options{}
	logLevel            = len(log.AllLevels) - 2
	dockerEngineAddress = defaultDockerEngineAddress
	listen              = "/run/docker/plugins/hello.sock"

	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

type backend struct {
	name    string
	docker  client.APIClient
	service hello.Service
	options hello.Options
	data    chan<- []byte
}

func main() {

	backend := &backend{
		name: "hello",
		options: hello.Options{
			CheckLeaderInterval: 10 * time.Second,
		},
	}

	// Top level main command...  all subcommands are designed to create the watch function
	// for the watcher, except 'version' subcommand.  After the subcommand completes, the
	// post run then begins execution of the actual watcher.
	cmd := &cobra.Command{
		Use:   backend.name,
		Short: "test plugin",
		PersistentPreRunE: func(c *cobra.Command, args []string) error {

			if logLevel > len(log.AllLevels)-1 {
				logLevel = len(log.AllLevels) - 1
			} else if logLevel < 0 {
				logLevel = 0
			}
			log.SetLevel(log.AllLevels[logLevel])

			if c.Use == "version" {
				return nil
			}

			// Docker client
			d, err := hello.NewDockerClient(dockerEngineAddress, &tlsOptions)
			if err != nil {
				return err
			}
			backend.docker = d
			backend.options.DockerEngineAddress = dockerEngineAddress
			backend.options.DockerTlsOptions = tlsOptions
			log.Infoln("Connected to engine")

			return nil
		},

		// After the subcommand completed we start the main part...
		PersistentPostRunE: func(c *cobra.Command, args []string) error {
			switch c.Use {
			case "version", "client":
				return nil
			default:
			}

			log.Infoln("Starting httpd")
			_, waitHTTP, err := runHTTP(listen, backend)
			if err != nil {
				panic(err)
			}
			log.Infoln("Started httpd")

			<-waitHTTP
			return nil
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print build version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s (revision %s)\n", Version, Revision)
		},
	})

	// This tester is used to build TWO different plugins. So we pass the name from the command line to make it
	// easy to tell which is which.
	cmd.PersistentFlags().StringVar(&backend.name, "name", backend.name, "name of the plugin")

	cmd.PersistentFlags().StringVar(&listen, "listen", listen, "listen address (unix or tcp)")
	cmd.PersistentFlags().StringVar(&dockerEngineAddress, "dockerEngineAddress", defaultDockerEngineAddress, "Docker dockerEngineAddress")
	cmd.PersistentFlags().StringVar(&tlsOptions.CAFile, "tlscacert", "", "TLS CA cert")
	cmd.PersistentFlags().StringVar(&tlsOptions.CertFile, "tlscert", "", "TLS cert")
	cmd.PersistentFlags().StringVar(&tlsOptions.KeyFile, "tlskey", "", "TLS key")
	cmd.PersistentFlags().BoolVar(&tlsOptions.InsecureSkipVerify, "tlsverify", true, "True to skip TLS")
	cmd.PersistentFlags().IntVar(&logLevel, "log", logLevel, "Logging level. 0 is least verbose. Max is 5")

	// The subcommand is run only to set up the data source.
	run := runCommand(backend)
	backend.options.BindFlags(run.Flags())

	client := clientCommand(backend)

	cmd.AddCommand(run, client)

	err := cmd.Execute()
	if err != nil {
		panic(err)
	}
}
