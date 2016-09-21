package groupserver

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	group_plugin "github.com/docker/libmachete/plugin/group"
	"github.com/docker/libmachete/plugin/group/swarm"
	"github.com/docker/libmachete/spi/instance"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"time"
)

// Run starts the group server, blocking until it exits.
func Run(pluginLookup func(string) (instance.Plugin, error)) {
	var port uint
	var cluster string

	rootCmd := &cobra.Command{Use: "groupctl"}

	rootCmd.PersistentFlags().UintVar(&port, "port", 8888, "Port the server listens on")
	rootCmd.PersistentFlags().StringVar(
		&cluster,
		"cluster",
		"default",
		"Machete cluster ID, used to isolate separate infrastructures")

	run := func(cmd *cobra.Command, args []string) {
		log.Infoln("Starting server on port", port)

		router := mux.NewRouter()
		router.StrictSlash(true)

		dockerClient, err := newDockerClient("unix:///var/run/docker.sock", nil)
		if err != nil {
			log.Error(err)
		}

		grp := group_plugin.NewGroupPlugin(
			pluginLookup,
			swarm.NewSwarmProvisionHelper(dockerClient),
			10*time.Second)

		adapter := httpAdapter{plugin: grp}

		router.HandleFunc("/Watch", outputHandler(adapter.watch)).Methods("POST")
		router.HandleFunc("/Unwatch/{id}", outputHandler(adapter.unwatch)).Methods("POST")
		router.HandleFunc("/Inspect/{id}", outputHandler(adapter.inspect)).Methods("POST")
		router.HandleFunc("/DescribeUpdate", outputHandler(adapter.describeUpdate)).Methods("POST")
		router.HandleFunc("/UpdateGroup", outputHandler(adapter.updateGroup)).Methods("POST")
		router.HandleFunc("/DestroyGroup/{id}", outputHandler(adapter.destroyGroup)).Methods("POST")

		http.Handle("/", router)

		if err := http.ListenAndServe(fmt.Sprintf(":%v", port), router); err != nil {
			log.Error(err)
		}
	}

	rootCmd.AddCommand(&cobra.Command{Use: "run", Run: run})

	err := rootCmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(-1)
	}
}
