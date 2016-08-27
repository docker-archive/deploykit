package main

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/controller"
	"github.com/docker/libmachete/controller/util"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
)

func runHTTP(driverDir string, listen string, backend *backend) (chan<- struct{}, <-chan error, error) {
	log.Infoln("Starting watcher at:", listen)
	return util.StartServer(listen, handlers(backend),
		func() error {
			log.Infoln("Shutting down...")
			if backend.watcher != nil {
				log.Infoln("Stopping the watcher")
				backend.watcher.Stop()
			}
			<-backend.watcherDone
			log.Infoln("Watcher stopped")
			return nil
		})
}

func noError(err error, resp http.ResponseWriter) bool {
	if err != nil {
		log.Warningln("error=", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return false
	}
	return true
}

type info struct {
	controller.Info
	Running bool `json:"running"`
}

// handler returns a http handler
func handlers(backend *backend) http.Handler {
	router := mux.NewRouter()

	// Get the info
	router.HandleFunc("/v1/info",
		func(resp http.ResponseWriter, req *http.Request) {
			log.Infoln("Request for info")
			info := info{
				Info: controller.Info{
					Name:        "config watcher",
					DriverName:  "watcher",
					DriverType:  "watcher",
					Version:     Version,
					Revision:    Revision,
					Description: "Docker-implemented scaler for managing groups of nodes in swarm.",
					Namespace:   "watcher",
					Image:       "libmachete/watcher", // TODO(chungers) - externalize this as a flag
				},
				Running: backend.watcher != nil && backend.watcher.Running(),
			}

			buff, err := json.Marshal(&info)
			if noError(err, resp) {
				_, err = resp.Write(buff)
				if noError(err, resp) {
					return
				}
			}
			log.Warningln("err=", err)
			return
		}).Methods("GET")

	// RPC - style API like Docker Plugins

	// Start begins actual work
	router.HandleFunc("/v1/watcher.Update",
		func(resp http.ResponseWriter, req *http.Request) {
			log.Infoln("Update requested via http")
			buff, err := ioutil.ReadAll(req.Body)
			if noError(err, resp) {
				if backend.data != nil {
					backend.data <- buff
				}
				log.Infoln("Dispatched configuration.")
				return
			}

			resp.WriteHeader(http.StatusAccepted)
			return
		}).Methods("POST")

	// GetState - returns current state
	router.HandleFunc("/v1/watcher.Get",
		func(resp http.ResponseWriter, req *http.Request) {
			log.Infoln("State requested via http")
			if backend.watcher != nil {
				if state, err := backend.watcher.GetState(); noError(err, resp) {
					buff, err := json.Marshal(state)
					if noError(err, resp) {
						resp.Write(buff)
					}
					return
				}
			}
			resp.WriteHeader(http.StatusNoContent)
			return
		}).Methods("GET")
	return router
}
