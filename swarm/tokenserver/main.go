package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"net/http"
	"os"
)

var (
	// Version is the build release identifier.
	Version = "Unspecified"

	// Revision is the build source control revision.
	Revision = "Unspecified"
)

func run(port uint, dockerSocket string) {
	makeHandler := func(getManager bool) func(w http.ResponseWriter, r *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			sendError := func(status int, err error) {
				w.WriteHeader(status)
				w.Write([]byte(err.Error()))
			}

			cli, err := client.NewClient(fmt.Sprintf("unix://%s", dockerSocket), "v1.24", nil, map[string]string{})
			if err != nil {
				sendError(http.StatusBadGateway, err)
				return
			}

			swarmStatus, err := cli.SwarmInspect(context.Background())
			if err != nil {
				sendError(http.StatusBadGateway, err)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "text/plain")

			var body string
			if getManager {
				body = swarmStatus.JoinTokens.Manager
			} else {
				body = swarmStatus.JoinTokens.Worker
			}

			w.Write([]byte(body))
		}
	}

	http.HandleFunc("/token/manager", makeHandler(true))
	http.HandleFunc("/token/worker", makeHandler(false))

	fmt.Printf("Listening on port %d\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))

	router := mux.NewRouter()
	router.StrictSlash(true)
}

func main() {
	rootCmd := &cobra.Command{
		Use: "tokenserver",
		Long: `tokenserver proxies the Docker Engine /swarm endpoint.
		This is useful for accessing Swarm join tokens without exposing the entire Docker Remote API.`,
	}

	var port uint
	var dockerSocket string
	rootCmd.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "run the token server",
		Run: func(cmd *cobra.Command, args []string) {
			run(port, dockerSocket)
		},
	})
	rootCmd.Flags().UintVar(&port, "port", 8889, "Port the server listens on")
	rootCmd.Flags().StringVar(&dockerSocket, "docker-socket", "/var/run/docker.sock", "Docker daemon socket path")

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print build version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s (revision %s)\n", Version, Revision)
		},
	})

	err := rootCmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(-1)
	}
}
