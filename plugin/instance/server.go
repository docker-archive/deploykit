package instance

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/spi/instance"
	"github.com/gorilla/mux"
	"net/http"
)

type routerAttachment interface {
	attachTo(router *mux.Router)
}

// NewHandler creates an HTTP handler for the machete server.
func NewHandler(instanceProvisioner instance.Plugin, info func() interface{}) http.Handler {
	// TODO(wfarner): As more provisioner types are added, consider optionally mounting them to allow serving
	// partial SPI implementations (e.g. network-only provisioner).

	router := mux.NewRouter()
	router.StrictSlash(true)

	// Sanity / info endpoint
	router.HandleFunc("/v1/info",
		func(resp http.ResponseWriter, req *http.Request) {
			log.Infoln("Request for info")
			buff, err := json.MarshalIndent(info(), "  ", "  ")
			if noError(err, resp) {
				_, err = resp.Write(buff)
				if noError(err, resp) {
					return
				}
			}
			return
		}).Methods("GET")

	// Plugin methods
	// TOOD(chungers) -- these need to look like Driver HTTP-RPC style (e.g. VolumeDriver)
	attachments := map[string]routerAttachment{
		"/instance": &instanceHandler{provisioner: instanceProvisioner},
	}

	for path, attachment := range attachments {
		attachment.attachTo(router.PathPrefix(path).Subrouter())
	}

	return router
}

func noError(err error, resp http.ResponseWriter) bool {
	if err != nil {
		log.Warningln("error=", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return false
	}
	return true
}
