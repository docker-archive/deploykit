package http

import (
	"github.com/docker/libmachete/api"
	"github.com/gorilla/mux"
	"net/http"
)

type swarmHandler struct {
	swarms api.Swarms
}

func getSwarmID(req *http.Request) api.SwarmID {
	return api.SwarmID(mux.Vars(req)["key"])
}

func (h *swarmHandler) getAll(req *http.Request) (interface{}, *api.Error) {
	return h.swarms.ListIDs()
}

func (h *swarmHandler) getOne(req *http.Request) (interface{}, *api.Error) {
	return h.swarms.Get(getSwarmID(req))
}

func (h *swarmHandler) create(req *http.Request) (interface{}, *api.Error) {
	events, err := h.swarms.Create(
		provisionerNameFromQuery(req),
		orDefault(req.URL.Query().Get("credentials"), "default"),
		orDefault(req.URL.Query().Get("template"), "default"),
		req.Body,
		api.ContentTypeJSON)
	return handleEventResponse(req, events, err)
}

func (h *swarmHandler) delete(req *http.Request) (interface{}, *api.Error) {
	events, err := h.swarms.Delete(orDefault(req.URL.Query().Get("credentials"), "default"), getSwarmID(req))
	return handleEventResponse(req, events, err)
}

func (h *swarmHandler) attachTo(router *mux.Router) {
	router.HandleFunc("/", outputHandler(h.getAll)).Methods("GET")
	router.HandleFunc("/{key}", outputHandler(h.getOne)).Methods("GET")
	router.HandleFunc("/{key}", outputHandler(h.create)).Methods("POST")
	router.HandleFunc("/{key}", outputHandler(h.delete)).Methods("DELETE")
}
