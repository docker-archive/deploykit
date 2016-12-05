package server

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"gopkg.in/tylerb/graceful.v1"
	"net"
	"net/http"
	"time"

	"github.com/docker/infrakit/pkg/rpc/plugin"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
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
func StartPluginAtPath(socketPath string, receiver VersionedInterface) (Stoppable, error) {
	server := rpc.NewServer()
	server.RegisterCodec(json2.NewCodec(), "application/json")

	if err := server.RegisterService(receiver, ""); err != nil {
		return nil, err
	}

	if err := server.RegisterService(plugin.Plugin{Spec: receiver.ImplementedInterface()}, ""); err != nil {
		return nil, err
	}

	httpLog := log.New()
	httpLog.Level = log.GetLevel()

	handler := loggingHandler{handler: server}
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
