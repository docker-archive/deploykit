package server

import (
	"fmt"
	"github.com/docker/libmachete/spi/instance"
	"github.com/gorilla/mux"
	"net/http"
)

type routerAttachment interface {
	attachTo(router *mux.Router)
}

// NewHandler creates an HTTP handler for the machete server.
func NewHandler(instanceProvisioner instance.Provisioner) http.Handler {
	// TODO(wfarner): As more provisioner types are added, consider optionally mounting them to allow serving
	// partial SPI implementations (e.g. network-only provisioner).

	attachments := map[string]routerAttachment{
		"/instance": &instanceHandler{provisioner: instanceProvisioner},
	}

	router := mux.NewRouter()
	router.StrictSlash(true)

	for path, attachment := range attachments {
		attachment.attachTo(router.PathPrefix(path).Subrouter())
	}

	return router
}

// RunServer starts an API server that delegates operations to a provisioner implementation.
func RunServer(port uint, instanceProvisioner instance.Provisioner) error {
	handler := NewHandler(instanceProvisioner)
	http.Handle("/", handler)
	return http.ListenAndServe(fmt.Sprintf(":%v", port), handler)
}
