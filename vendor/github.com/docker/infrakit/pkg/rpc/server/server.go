package server

import (
	"fmt"
	"net"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	rpc_server "github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
	"gopkg.in/tylerb/graceful.v1"
	"net/http/httptest"
	"net/http/httputil"
)

// Stoppable support proactive stopping, and blocking until stopped.
type Stoppable interface {
	Stop()
	AwaitStopped()
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

type loggingHandler struct {
	handler http.Handler
}

func (h loggingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	requestData, err := httputil.DumpRequest(req, true)
	if err == nil {
		log.Debugf("Received request %s", string(requestData))
	} else {
		log.Error(err)
	}

	recorder := httptest.NewRecorder()

	h.handler.ServeHTTP(recorder, req)

	responseData, err := httputil.DumpResponse(recorder.Result(), true)
	if err == nil {
		log.Debugf("Sending response %s", string(responseData))
	} else {
		log.Error(err)
	}

	w.WriteHeader(recorder.Code)
	recorder.Body.WriteTo(w)
}

// A VersionedInterface identifies which Interfaces a plugin supports.
type VersionedInterface interface {
	// ImplementedInterface returns the interface being provided.
	ImplementedInterface() spi.InterfaceSpec
}

// StartPluginAtPath starts an HTTP server listening on a unix socket at the specified path.
// Returns a Stoppable that can be used to stop or block on the server.
func StartPluginAtPath(socketPath string, receiver VersionedInterface, more ...VersionedInterface) (Stoppable, error) {
	server := rpc.NewServer()
	server.RegisterCodec(json2.NewCodec(), "application/json")

	interfaces := []spi.InterfaceSpec{}

	if err := server.RegisterService(receiver, ""); err != nil {
		return nil, err
	}

	interfaces = append(interfaces, receiver.ImplementedInterface())

	// Additional interfaces to publish
	for _, obj := range more {
		// the object can export 0 or more methods.  In any case we show the implemented interface
		interfaces = append(interfaces, obj.ImplementedInterface())
		if err := server.RegisterService(obj, ""); err != nil {
			log.Warningln(err)
		}
	}

	// TODO - deprecate this in favor of the more information-rich /info/api.json endpoint
	if err := server.RegisterService(rpc_server.Handshake(interfaces), ""); err != nil {
		return nil, err
	}

	// info handler
	info, err := NewPluginInfo(receiver)
	if err != nil {
		return nil, err
	}

	httpLog := log.New()
	httpLog.Level = log.GetLevel()

	router := mux.NewRouter()
	router.Handle(rpc_server.InfoURL, info)
	router.Handle("/", server)

	handler := loggingHandler{handler: router}
	gracefulServer := graceful.Server{
		Timeout: 10 * time.Second,
		Server:  &http.Server{Addr: fmt.Sprintf("unix://%s", socketPath), Handler: handler},
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}

	log.Infof("Listening at: %s", socketPath)

	go func() {
		err := gracefulServer.Serve(listener)
		if err != nil {
			log.Warn(err)
		}
	}()

	return &stoppableServer{server: &gracefulServer}, nil
}
