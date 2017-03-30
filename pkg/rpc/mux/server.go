package mux

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/rpc"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"gopkg.in/tylerb/graceful.v1"
)

// NewServer returns a tcp server listening at the listen address (e.g. ':8080'), or error
func NewServer(listen string, plugins func() discovery.Plugins) (rpc_server.Stoppable, error) {

	proxy := NewReverseProxy(plugins)
	server := &graceful.Server{
		Timeout: 10 * time.Second,
		Server:  &http.Server{Addr: listen, Handler: proxy},
	}

	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, err
	}

	log.Info("Listening", "listen", listen)

	go func() {
		defer log.Warn("listener stopped")

		err := server.Serve(listener)
		if err != nil {
			log.Warn("err", "err", err)
			return
		}
	}()
	return &stoppableServer{server: server}, nil
}

type stoppableServer struct {
	server *graceful.Server
}

func (s *stoppableServer) Stop() {
	s.server.Stop(10 * time.Second)
}

func (s *stoppableServer) AwaitStopped() {
	<-s.server.StopChan()
}

func (s *stoppableServer) Wait() <-chan struct{} {
	return s.server.StopChan()
}

type loggingHandler struct {
	handler http.Handler
}

func (h loggingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if strings.Index(req.URL.Path, rpc.URLEventsPrefix) == 0 {
		// this is an event stream... do not record
		log.Debug("Requesting event stream", "url", req.URL)
		h.handler.ServeHTTP(w, req)
		return
	}

	requestData, err := httputil.DumpRequest(req, true)
	if err == nil {
		log.Debug("Received", "request", string(requestData))
	} else {
		log.Error("err", "err", err)
	}

	recorder := httptest.NewRecorder()

	h.handler.ServeHTTP(recorder, req)

	responseData, err := httputil.DumpResponse(recorder.Result(), true)
	if err == nil {
		log.Debug("Responding", "response", string(responseData))
	} else {
		log.Error("err", "err", err)
	}

	w.WriteHeader(recorder.Code)
	recorder.Body.WriteTo(w)
}
