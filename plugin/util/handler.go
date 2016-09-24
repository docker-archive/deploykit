package util

import (
	"encoding/json"
	"net/http"
	"runtime/debug"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/plugin"
	"github.com/gorilla/mux"
)

func BuildHandler(endpoints []func() (plugin.Endpoint, plugin.Handler)) http.Handler {

	router := mux.NewRouter()
	router.StrictSlash(true)

	for _, f := range endpoints {

		endpoint, serve := f()

		ep, err := GetHTTPEndpoint(endpoint)
		if err != nil {
			panic(err) // This is system initialization so we have to panic
		}

		router.HandleFunc(ep.Path, func(resp http.ResponseWriter, req *http.Request) {
			defer func() {
				req.Body.Close()
				if err := recover(); err != nil {
					log.Errorf("%s: %s", err, debug.Stack())
					respond(http.StatusInternalServerError, err, resp)
					return
				}
			}()
			result, err := serve(mux.Vars(req), req.Body)
			if err != nil {
				respond(http.StatusInternalServerError, nil, resp)
				return
			}
			respond(http.StatusOK, result, resp)
			return
		}).Methods(strings.ToUpper(ep.Method))
	}
	return router
}

func respond(status int, body interface{}, resp http.ResponseWriter) {
	resp.WriteHeader(status)
	if body != nil {

		switch body := body.(type) {
		}
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			status = http.StatusInternalServerError
			bodyJSON = []byte(`{"error": "Internal error"`)
			log.Warn("Failed to marshal response body %v: %s", body, err.Error())
		}
		resp.Header().Set("Content-Type", "application/json")
		resp.Write(bodyJSON)
	}
}
