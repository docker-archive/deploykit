package server

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"time"

	broker "github.com/docker/infrakit/pkg/broker/server"
	logutil "github.com/docker/infrakit/pkg/log"
	rpc_base "github.com/docker/infrakit/pkg/rpc"
	rpc_server "github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/types"
	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
	"gopkg.in/tylerb/graceful.v1"
)

var (
	log    = logutil.New("module", "rpc/server")
	debugV = logutil.V(1000)
)

// Stoppable support proactive stopping, and blocking until stopped.
type Stoppable interface {
	Stop()
	AwaitStopped()
	Wait() <-chan struct{}
}

type stoppableServer struct {
	server *graceful.Server
}

func (s *stoppableServer) Stop() {
	s.server.Stop(10 * time.Second)
}

func (s *stoppableServer) Wait() <-chan struct{} {
	return s.server.StopChan()
}

func (s *stoppableServer) AwaitStopped() {
	<-s.server.StopChan()
}

type loggingHandler struct {
	handler      http.Handler
	listen       []string
	discoverPath string
}

func (h loggingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	requestData, err := httputil.DumpRequest(req, true)
	if err == nil {
		log.Debug("Server RECEIVE", "payload", string(requestData), "V", debugV,
			"url", fmt.Sprintf("%v", req.URL), "listen", h.listen, "discoverPath", h.discoverPath)
	} else {
		log.Error("Server RECEIVE", "err", err, "url", fmt.Sprintf("%v", req.URL))
	}

	recorder := rpc_server.NewRecorder()

	h.handler.ServeHTTP(recorder, req)

	responseData, err := httputil.DumpResponse(recorder.Result(), true)
	if err == nil {
		log.Debug("Server REPLY", "payload", string(responseData), "V", debugV,
			"url", fmt.Sprintf("%v", req.URL), "listen", h.listen, "discoverPath", h.discoverPath)
	} else {
		log.Error("Server REPLY", "err", err, "url", fmt.Sprintf("%v", req.URL),
			"url", fmt.Sprintf("%v", req.URL), "listen", h.listen, "discoverPath", h.discoverPath)
	}

	w.WriteHeader(recorder.Code)
	recorder.Body.WriteTo(w)
}

// A VersionedInterface identifies which Interfaces a plugin supports.
type VersionedInterface interface {
	// ImplementedInterface returns the interface being provided.
	ImplementedInterface() spi.InterfaceSpec
	// Objects returns the names of objects
	Objects() []rpc_base.Object
}

// StartListenerAtPath starts an HTTP server listening on tcp port with discovery entry at specified path.
// Returns a Stoppable that can be used to stop or block on the server.
func StartListenerAtPath(listen []string, discoverPath string,
	receiver VersionedInterface, more ...VersionedInterface) (Stoppable, error) {
	return startAtPath(listen, discoverPath, receiver, more...)
}

// StartPluginAtPath starts an HTTP server listening on a unix socket at the specified path.
// Returns a Stoppable that can be used to stop or block on the server.
func StartPluginAtPath(socketPath string, receiver VersionedInterface, more ...VersionedInterface) (Stoppable, error) {
	return startAtPath(nil, socketPath, receiver, more...)
}

func startAtPath(listen []string, discoverPath string,
	receiver VersionedInterface, more ...VersionedInterface) (Stoppable, error) {

	df, err := os.Stat(discoverPath)
	if os.IsExist(err) {
		log.Error("socket exists", "path", discoverPath, "found", df)
		fmt.Printf("Socket/ port file found at %v.  Please clean up if no other processes are listening.\n", discoverPath)
		os.Exit(-1)
	}

	server := rpc.NewServer()
	server.RegisterCodec(json2.NewCodec(), "application/json")

	targets := append([]VersionedInterface{receiver}, more...)

	objects := map[spi.InterfaceSpec]func() []rpc_base.Object{}
	for _, t := range targets {

		objects[t.ImplementedInterface()] = t.Objects

		if err := server.RegisterService(t, ""); err != nil {
			return nil, err
		}

		// polymorphic -- register additional interfaces
		// if pub, is := t.(event.Plugin); is {

		// 	t = rpc_event.PluginServer(pub)
		// 	interfaces[event.InterfaceSpec] = t.Types

		// 	if err := server.RegisterService(t, ""); err != nil {
		// 		return nil, err
		// 	}

		// 	log.Info("Object exported as event producer", "object", t)
		// }
	}
	// handshake service that can exchange interface versions with client
	if err := server.RegisterService(rpc_server.Handshake(objects), ""); err != nil {
		return nil, err
	}

	// a list of channels to close on stop
	stops := []chan struct{}{}

	// events handler
	events := broker.NewBroker()

	// wire up the publish event source channel to the plugin implementations
	for _, t := range targets {

		pub, is := t.(event.Publisher)
		if !is {
			continue
		}

		log.Info("Object is an event producer", "object", t, "discover", discoverPath)

		stop := make(chan struct{})
		stops = append(stops, stop)

		// We give one channel per source to provide some isolation.  This we won't have the
		// whole event bus stop just because one plugin closes the channel.
		eventChan := make(chan *event.Event)
		pub.PublishOn(eventChan)
		go func() {
			for {
				select {
				case event, ok := <-eventChan:
					if !ok {
						return
					}
					if event.Timestamp.IsZero() {
						event.Now()
					}
					events.Publish(event.Topic.String(), event, 1*time.Second)
				case <-stop:
					log.Info("Stopping event relay")
					return
				}
			}
		}()
	}

	// info handler
	info, err := NewPluginInfo(receiver)
	if err != nil {
		return nil, err
	}

	router := mux.NewRouter()
	router.HandleFunc(rpc_server.URLAPI, info.ShowAPI)
	router.HandleFunc(rpc_server.URLFunctions, info.ShowTemplateFunctions)

	// Disable this so that clients can connect/subscribe to streams before the topics
	// actually become available (dynamically added topics)
	// TODO(chungers) - make this an option somehow
	skipTopicValidation := true

	intercept := broker.Interceptor{
		Pre: func(topic string, headers map[string][]string) error {
			if skipTopicValidation {
				return nil
			}

			for _, target := range targets {
				if v, is := target.(event.Validator); is {
					if err := v.Validate(types.PathFromString(topic)); err == nil {
						return nil
					}
				}
			}
			return broker.ErrInvalidTopic(topic)
		},
		Do: events.ServeHTTP,
		Post: func(topic string) {
			log.Debug("Client left", "topic", topic, "V", logutil.V(100))
		},
	}
	router.HandleFunc(rpc_server.URLEventsPrefix, intercept.ServeHTTP)

	logger := loggingHandler{handler: server, listen: listen, discoverPath: discoverPath}
	router.Handle("/", logger)

	gracefulServer := graceful.Server{
		Timeout: 10 * time.Second,
	}

	var listener net.Listener

	if len(listen) > 0 {

		// TCP listener

		gracefulServer.Server = &http.Server{
			Addr:    listen[0],
			Handler: router,
		}
		l, err := net.Listen("tcp", listen[0])
		if err != nil {
			log.Error("error listening tcp", "err", err)
			return nil, err
		}
		listener = l

		advertise := listen[0]
		if len(listen) > 1 {
			advertise = listen[1]
		}
		if err := ioutil.WriteFile(discoverPath, []byte(fmt.Sprintf("tcp://%s", advertise)), 0644); err != nil {
			return nil, err
		}

		log.Info("Listening", "listen", listen, "discover", discoverPath)

	} else {

		// Unix Socket listener

		gracefulServer.Server = &http.Server{
			Addr:    fmt.Sprintf("unix://%s", discoverPath),
			Handler: router,
		}
		l, err := net.Listen("unix", discoverPath)
		if err != nil {
			log.Error("error listening unix", "err", err)
			return nil, err
		}
		listener = l
		log.Info("Listening", "discover", discoverPath)

	}

	go func() {

		// This is the core server loop.  Note that it will block and return
		// when signals are received.  We then perform cleanup after that
		// by closing all the channels to signal shutdown.

		err := gracefulServer.Serve(listener)
		if err != nil {
			log.Error("http server err", "err", err)
		}

		for _, ch := range stops {
			close(ch)
		}

		events.Stop()
		if len(listen) > 0 {
			os.Remove(discoverPath)
		}
	}()

	return &stoppableServer{server: &gracefulServer}, nil
}
