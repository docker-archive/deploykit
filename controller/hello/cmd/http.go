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

func runHTTP(listen string, backend *backend) (chan<- struct{}, <-chan error, error) {
	log.Infoln("Listening on:", listen)
	return util.StartServer(listen, handlers(backend),
		func() error {
			log.Infoln("Shutting down...")
			if backend.service != nil {
				log.Infoln("Stopping the service")
				backend.service.Stop()
			}
			backend.service.Wait()
			log.Infoln("Service stopped")
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
	State   interface{}
}

// handler returns a http handler
func handlers(backend *backend) http.Handler {
	router := mux.NewRouter()

	// Get the info
	router.HandleFunc("/v1/info",
		func(resp http.ResponseWriter, req *http.Request) {
			log.Infoln("Request for info")
			state, _ := backend.service.GetState()
			info := info{
				Info: controller.Info{
					Name:        "hello",
					DriverName:  "hello",
					DriverType:  "hello",
					Version:     Version,
					Revision:    Revision,
					Description: "Hello",
					Namespace:   "hello",
					Image:       "libmachete/hello",
				},
				Running: backend.service != nil,
				State:   state,
			}

			buff, err := json.Marshal(&info)
			if noError(err, resp) {
				_, err = resp.Write(buff)
				if noError(err, resp) {
					return
				}
			}
			return
		}).Methods("GET")

	// RPC - style API like Docker Plugins

	// Start begins actual work
	router.HandleFunc("/v1/hello.Update",
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
	router.HandleFunc("/v1/hello.Get",
		func(resp http.ResponseWriter, req *http.Request) {
			log.Infoln("State requested via http")
			if backend.service != nil {
				if state, err := backend.service.GetState(); noError(err, resp) {
					buff, err := json.Marshal(state)
					if noError(err, resp) {
						resp.Write(buff)
					}
					return
				}
			}
			resp.WriteHeader(http.StatusNoContent)
			return
		}).Methods("POST")
	return router
}
