package main

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/plugin/group"
	"github.com/docker/libmachete/server"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/group"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"os"
)

type httpAdapter struct {
	plugin group.Plugin
}

func getGroupID(req *http.Request) group.ID {
	return group.ID(mux.Vars(req)["id"])
}

func getConfiguration(req *http.Request) (group.Configuration, error) {
	grp := group.Configuration{}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return grp, fmt.Errorf("Failed to read body: %s", err)
	}

	err = json.Unmarshal(body, &grp)
	if err != nil {
		return grp, spi.NewError(spi.ErrBadInput, "Invalid group configuration: %s", err)
	}

	return grp, nil
}

func (h httpAdapter) watch(req *http.Request) (interface{}, error) {
	config, err := getConfiguration(req)
	if err != nil {
		return nil, err
	}

	return nil, h.plugin.WatchGroup(config)
}

func (h httpAdapter) unwatch(req *http.Request) (interface{}, error) {
	id := getGroupID(req)
	if len(id) == 0 {
		return nil, spi.NewError(spi.ErrBadInput, "Group ID must not be blank")
	}

	return nil, h.plugin.UnwatchGroup(id)
}

func (h httpAdapter) inspect(req *http.Request) (interface{}, error) {
	id := getGroupID(req)
	if len(id) == 0 {
		return nil, spi.NewError(spi.ErrBadInput, "Group ID must not be blank")
	}

	desc, err := h.plugin.InspectGroup(id)
	if err != nil {
		return nil, err
	}

	return &desc, nil
}

func (h httpAdapter) describeUpdate(req *http.Request) (interface{}, error) {
	config, err := getConfiguration(req)
	if err != nil {
		return nil, err
	}

	desc, err := h.plugin.DescribeUpdate(config)
	if err != nil {
		return nil, err
	}

	return &desc, nil
}

func (h httpAdapter) updateGroup(req *http.Request) (interface{}, error) {
	config, err := getConfiguration(req)
	if err != nil {
		return nil, err
	}

	return nil, h.plugin.UpdateGroup(config)
}

func (h httpAdapter) destroyGroup(req *http.Request) (interface{}, error) {
	id := getGroupID(req)
	if len(id) == 0 {
		return nil, spi.NewError(spi.ErrBadInput, "Group ID must not be blank")
	}

	return nil, h.plugin.DestroyGroup(id)
}

func main() {
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

		grp, err := scaler.NewGroup()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		adapter := httpAdapter{plugin: grp}

		router.HandleFunc("/Watch", server.OutputHandler(adapter.watch)).Methods("POST")
		router.HandleFunc("/Unwatch/{id}", server.OutputHandler(adapter.unwatch)).Methods("POST")
		router.HandleFunc("/Inspect/{id}", server.OutputHandler(adapter.inspect)).Methods("POST")
		router.HandleFunc("/DescribeUpdate", server.OutputHandler(adapter.describeUpdate)).Methods("POST")
		router.HandleFunc("/UpdateGroup", server.OutputHandler(adapter.updateGroup)).Methods("POST")
		router.HandleFunc("/DestroyGroup/{id}", server.OutputHandler(adapter.destroyGroup)).Methods("POST")

		http.Handle("/", router)
		err = http.ListenAndServe(fmt.Sprintf(":%v", port), router)
		if err != nil {
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
