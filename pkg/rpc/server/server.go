package server

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"gopkg.in/tylerb/graceful.v1"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json"
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

// StartPluginAtPath starts an HTTP server listening on a unix socket at the specified path.
// Returns a Stoppable that can be used to stop or block on the server.
func StartPluginAtPath(socketPath string, receiver interface{}) (Stoppable, error) {
	server := rpc.NewServer()
	server.RegisterCodec(json.NewCodec(), "application/json")

	if err := server.RegisterService(receiver, ""); err != nil {
		return nil, err
	}

	httpLog := log.New()
	httpLog.Level = log.GetLevel()

	handler := handlers.LoggingHandler(httpLog.WriterLevel(log.DebugLevel), server)
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
