package main

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/controller/util"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

// In case of tcp (e.g. :8080) we leave a file in the directory to simulate
// the socket files for unix socket -- for discovery.
func registerForDiscovery(driverDir, listen string) error {
	if util.ProtocolFromListenString(listen) == "unix" {
		// In case of unix socket, there's already a socket file that can be discovered.
		return nil
	}
	p := filepath.Join(driverDir, listen)
	return ioutil.WriteFile(p, []byte(listen), 0644)
}

// Remove file for discovery.  No op for the unix socket case.
func deregisterForDiscovery(driverDir, listen string) error {
	if util.ProtocolFromListenString(listen) == "unix" {
		// In case of unix socket, there's already a socket file that can be discovered.
		return nil
	}
	p := filepath.Join(driverDir, listen)
	return os.Remove(p)
}

func runHTTP(driverDir string, listen string, backend *backend) (chan<- struct{}, <-chan error, error) {
	log.Infoln("Starting controller at:", listen)
	stop, wait, err := util.StartServer(listen, handlers(backend),
		func() error {
			log.Infoln("Shutting down...")
			if backend.scaler != nil {
				log.Infoln("Stopping the instance watcher")
				backend.scaler.Stop()
			}
			return deregisterForDiscovery(driverDir, listen)
		})
	if err != nil {
		return nil, nil, err
	}
	if err := registerForDiscovery(driverDir, listen); err != nil {
		return nil, nil, err
	}
	return stop, wait, nil
}

func noError(err error, resp http.ResponseWriter) bool {
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		return false
	}
	return true
}

// handler returns a http handler
func handlers(backend *backend) http.Handler {
	router := mux.NewRouter()

	// Get the info
	router.HandleFunc("/v1/info",
		func(resp http.ResponseWriter, req *http.Request) {
			buff, err := json.Marshal(map[string]interface{}{
				"version":  Version,
				"revision": Revision,
				"scaler":   backend.scaler,
				"running":  backend.scaler != nil,
				"config":   json.RawMessage(backend.request),
			})
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
	router.HandleFunc("/v1/scaler.Start",
		func(resp http.ResponseWriter, req *http.Request) {
			log.Infoln("Start requested via http")
			buff, err := ioutil.ReadAll(req.Body)
			if noError(err, resp) {
				if backend.scaler == nil {
					backend.config <- buff
				}
				log.Infoln("Dispatched configuration. Starting.")
				return
			}

			resp.WriteHeader(http.StatusAccepted)
			return
		}).Methods("POST")

	// GetState - returns current state
	router.HandleFunc("/v1/scaler.GetState",
		func(resp http.ResponseWriter, req *http.Request) {
			log.Infoln("State requested via http")
			if backend.scaler != nil {
				if buff, err := backend.scaler.GetState(); noError(err, resp) {
					resp.Write(buff)
					return
				}
			}
			resp.WriteHeader(http.StatusNoContent)
			return
		}).Methods("GET")
	return router
}
