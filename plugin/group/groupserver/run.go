package groupserver

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	group_plugin "github.com/docker/libmachete/plugin/group"
	"github.com/docker/libmachete/plugin/group/swarm"
	"github.com/docker/libmachete/spi/instance"
	"github.com/gorilla/mux"
	"net/http"
	"time"
)

// Run starts the group server, blocking until it exits.
func Run(port uint, pluginLookup func(string) (instance.Plugin, error)) {
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
