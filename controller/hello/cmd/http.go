package main

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/controller"
	"github.com/docker/libmachete/controller/hello"
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
					Name:        backend.name,
					DriverName:  backend.name,
					DriverType:  backend.name,
					Version:     Version,
					Revision:    Revision,
					Description: backend.name,
					Namespace:   backend.name,
					Image:       "libmachete/" + backend.name,
				},
				Running: backend.service != nil,
				State:   state,
			}

			buff, err := json.MarshalIndent(&info, "  ", "  ")
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
	router.HandleFunc("/v1/"+backend.name+".Update",
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
	router.HandleFunc("/v1/"+backend.name+".GetState",
		func(resp http.ResponseWriter, req *http.Request) {
			log.Infoln("State requested via http")
			if backend.service != nil {
				if state, err := backend.service.GetState(); noError(err, resp) {
					buff, err := json.MarshalIndent(state, "  ", "  ")
					if noError(err, resp) {
						resp.Write(buff)
					}
					return
				}
			}
			resp.WriteHeader(http.StatusNoContent)
			return
		}).Methods("POST")

	// Discover another plugin
	router.HandleFunc("/v1/"+backend.name+".Discover",
		func(resp http.ResponseWriter, req *http.Request) {
			buff, err := ioutil.ReadAll(req.Body)
			if noError(err, resp) {
				p := &hello.Plugin{}
				err = json.Unmarshal(buff, p)
				if noError(err, resp) {
					discovered, err := backend.service.DiscoverPlugin(*p)
					if noError(err, resp) {
						buff, err = json.MarshalIndent(discovered, "  ", "  ")
						if noError(err, resp) {
							resp.Write(buff)
							return
						}
					}
				}
			}
			return
		}).Methods("POST")

	// Call another plugin
	router.HandleFunc("/v1/"+backend.name+".Call",
		func(resp http.ResponseWriter, req *http.Request) {
			buff, err := ioutil.ReadAll(req.Body)
			if noError(err, resp) {
				call := &hello.PluginCall{}
				err = json.Unmarshal(buff, call)
				if noError(err, resp) {
					out, err := backend.service.CallPlugin(*call)
					if noError(err, resp) {
						buff, err = json.MarshalIndent(out, "  ", "  ")
						if noError(err, resp) {
							resp.Write(buff)
							return
						}
					}
				}
			}
			return
		}).Methods("POST")

	return router
}
