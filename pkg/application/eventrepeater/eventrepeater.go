package eventrepeater

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	event_rpc "github.com/docker/infrakit/pkg/rpc/event"
	"github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/types"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/mux"
	"gopkg.in/tylerb/graceful.v1"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// NewEventRepeater creates an event repeater application.
func NewEventRepeater(eSource string, eSink string, protocol string, allowall bool) *EventRepeater {
	sub, err := event_rpc.NewClient(eSource)
	if err != nil {
		log.Errorf("Cannot Create Event Client :%v", err)
		return nil
	}
	pub, err := newSinkClient(protocol, eSink)
	if err != nil {
		log.Errorf("Cannot Create Sink Client protocol :%v Server :%v :%v", protocol, eSink, err)
		return nil
	}
	e := &EventRepeater{
		EventSource:   eSource,
		EventSink:     eSink,
		Events:        make(map[string]*RepeatRule),
		Protocol:      protocol,
		stop:          make(chan bool),
		sourceEClient: sub.(event.Subscriber),
		sinkEClient:   pub,
		allowAll:      allowall,
		eventListMt:   new(sync.Mutex),
	}
	return e
}

func newSinkClient(protocol string, broker string) (interface{}, error) {
	switch protocol {
	case "mqtt":
		opts := MQTT.NewClientOptions()
		opts.AddBroker(broker)
		c := MQTT.NewClient(opts)
		if token := c.Connect(); token.Wait() && token.Error() != nil {
			return nil, token.Error()
		}
		return c, nil
	case "stderr":
		log.Info("Print Event messages to stderr. This is for debug. ")
		return nil, nil
	default:
		return nil, fmt.Errorf("Unkown sink protocol %s", protocol)
	}
}

// EventRepeater struct
type EventRepeater struct {
	EventSource   string
	EventSink     string
	Events        map[string]*RepeatRule
	Protocol      string
	stop          chan bool
	eventAdd      chan bool
	eventDel      chan bool
	sourceEClient event.Subscriber
	sinkEClient   interface{}
	allowAll      bool
	eventListMt   *sync.Mutex
}

// RepeatRule for each event
type RepeatRule struct {
	SourceTopic  string
	SinkTopic    string
	SourceStopCh chan<- struct{}
	SinkStopCh   chan bool
	SourceStream <-chan *event.Event
}

type messageData struct {
	SourceTopic string `json:"sourcetopic, omitempty"`
	SinkTopic   string `json:"sinktopic, omitempty"`
}

func (e EventRepeater) addEvent(sourcesTopic string, sinkTopic string) error {
	if sourcesTopic == "" {
		return fmt.Errorf("Error: %s", "You must have a topic of source for add repeat event.")
	}
	if sinkTopic == "" {
		sinkTopic = sourcesTopic
	}
	log.Debugf("Add event %s as %s", sourcesTopic, sinkTopic)
	e.eventListMt.Lock()
	defer e.eventListMt.Unlock()
	if _, ok := e.Events[sourcesTopic]; ok {
		return fmt.Errorf("Error: %s %s", "Topic already exist. :", sourcesTopic)
	}
	stream, stop, err := e.sourceEClient.SubscribeOn(types.PathFromString(sourcesTopic))
	if err != nil {
		return err
	}
	e.Events[sourcesTopic] = &RepeatRule{
		SourceTopic:  sourcesTopic,
		SinkTopic:    sinkTopic,
		SourceStopCh: stop,
		SinkStopCh:   make(chan bool),
		SourceStream: stream,
	}
	go e.publishToSink(e.Events[sourcesTopic])
	return nil
}

func (e EventRepeater) delEvent(sourcesTopic string) error {
	if sourcesTopic == "" {
		return fmt.Errorf("Error: %s", "You must have a topic of source for delete repeat event.")
	}
	log.Debugf("Delete event %s", sourcesTopic)
	e.eventListMt.Lock()
	defer e.eventListMt.Unlock()
	if _, ok := e.Events[sourcesTopic]; !ok {
		return fmt.Errorf("Error: %s %s", "There is no registerd topic. :", sourcesTopic)
	}
	e.Events[sourcesTopic].SinkStopCh <- true
	close(e.Events[sourcesTopic].SourceStopCh)
	delete(e.Events, sourcesTopic)
	return nil
}

// Stop EventRepeater server
func (e EventRepeater) Stop() {
	e.stop <- true
}

func (e EventRepeater) publishToSink(rr *RepeatRule) error {
	for {
		select {
		case <-rr.SinkStopCh:
			return nil
		case s, ok := <-rr.SourceStream:
			if !ok {
				log.Info("Server disconnected", "topic", rr.SourceTopic)
				return nil
			}
			buff, err := s.Bytes()
			if err != nil {
				return err
			}
			switch e.Protocol {
			case "mqtt":
				if rr.SinkTopic == "." {
					e.sinkEClient.(MQTT.Client).Publish(s.Topic.String(), 0, false, buff)
				} else {
					e.sinkEClient.(MQTT.Client).Publish(rr.SinkTopic, 0, false, buff)
				}
			case "stderr":
				if rr.SinkTopic == "." {
					log.Infof("Publish subtopic %s gettopic %v pubtopic %v message %s\n", rr.SourceTopic, s.Topic, s.Topic, buff)
				} else {
					log.Infof("Publish subtopic %s gettopic %v pubtopic %v message %s\n", rr.SourceTopic, s.Topic, rr.SinkTopic, buff)
				}
			}
		}
	}
}

func (e EventRepeater) eventsReqHandler(w http.ResponseWriter, r *http.Request) {
	var body []byte
	var err error
	if r.Method != "GET" {
		body, err = ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		if err := r.Body.Close(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	err = e.eventUpdate(r.Method, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	return
}

func (e EventRepeater) eventUpdate(method string, body []byte) error {
	var dataStruct []messageData
	err := json.Unmarshal(body, &dataStruct)
	if err != nil {
		return err
	}
	log.Debugf("Update message op %v Data %v \n", method, dataStruct)
	switch method {
	case "POST":
		for _, d := range dataStruct {
			log.Debugf("Add message %v \n", d)
			err := e.addEvent(d.SourceTopic, d.SinkTopic)
			if err != nil {
				return err
			}
		}
	case "DELETE":
		for _, d := range dataStruct {
			err := e.delEvent(d.SourceTopic)
			if err != nil {
				return err
			}
		}
	case "PUT":
		for _, d := range dataStruct {
			err := e.delEvent(d.SourceTopic)
			if err != nil {
				return err
			}
			err = e.addEvent(d.SourceTopic, d.SinkTopic)
			if err != nil {
				return err
			}
		}
	case "GET":
	default:
		log.Warnf("Unknown operation\n")
	}
	return nil
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
	handler http.Handler
}

//Serve : Make listener or unix socket, listening and serve REST server
func (e EventRepeater) Serve(discoverPath string, listen string) (server.Stoppable, error) {
	if e.allowAll {
		e.addEvent(".", "")
	}
	r := mux.NewRouter().StrictSlash(true)
	r.Path("/events").HandlerFunc(e.eventsReqHandler)
	gracefulServer := graceful.Server{
		Timeout: 10 * time.Second,
	}
	var listener net.Listener
	if listen != "" {
		gracefulServer.Server = &http.Server{
			Addr:    listen,
			Handler: r,
		}
		l, err := net.Listen("tcp", listen)
		if err != nil {
			return nil, err
		}
		listener = l
		if err := ioutil.WriteFile(discoverPath, []byte(fmt.Sprintf("tcp://%s", listen)), 0644); err != nil {
			return nil, err
		}
		log.Infof("Listening at: %s, discoverable at %s", listen, discoverPath)
	} else {
		gracefulServer.Server = &http.Server{
			Addr:    fmt.Sprintf("unix://%s", discoverPath),
			Handler: r,
		}
		l, err := net.Listen("unix", discoverPath)
		if err != nil {
			fmt.Printf("%s\n", err)
			return nil, err
		}
		listener = l
		log.Infof("Listening at: %s", discoverPath)
	}
	go func() {
		err := gracefulServer.Serve(listener)
		if err != nil {
			log.Warn(err)
		}
		if listen != "" {
			os.Remove(discoverPath)
		}
	}()
	return &stoppableServer{server: &gracefulServer}, nil
}
