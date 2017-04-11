package main

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	event_rpc "github.com/docker/infrakit/pkg/rpc/event"
	"github.com/docker/infrakit/pkg/spi/application"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"sync"
)

// NewEventRepeater creates an event repeater application.
func NewEventRepeater(eSource string, eSink string, protocol string, allowall bool) application.Plugin {
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
	e := &eventRepeater{
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
	go e.serve()
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

type eventRepeater struct {
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

func (e eventRepeater) Validate(applicationProperties *types.Any) error {
	return nil
}

func (e eventRepeater) Healthy(applicationProperties *types.Any) (application.Health, error) {
	return application.Healthy, nil
}

func (e eventRepeater) addEvent(sourcesTopic string, sinkTopic string) error {
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

func (e eventRepeater) delEvent(sourcesTopic string) error {
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

func (e eventRepeater) Stop() {
	e.stop <- true
}

func (e eventRepeater) publishToSink(rr *RepeatRule) error {
	templateURL := "str://{{.}}"
	engine, err := template.NewTemplate(templateURL, template.Options{})
	if err != nil {
		return err
	}
	for {
		select {
		case <-rr.SinkStopCh:
			return nil
		case s, ok := <-rr.SourceStream:
			if !ok {
				log.Info("Server disconnected", "topic", rr.SourceTopic)
				return nil
			}
			buff, err := engine.Render(s)
			if err != nil {
				return err
			}
			switch e.Protocol {
			case "mqtt":
				e.sinkEClient.(MQTT.Client).Publish(rr.SinkTopic, 0, false, buff)
			case "stderr":
				log.Infof("Publish subtopic %s gettopic %v pubtopic %v message %s\n", rr.SourceTopic, s.Topic, rr.SinkTopic, buff)
			}
		}
	}
}

func (e eventRepeater) Update(message *application.Message) error {
	var dataStruct []messageData
	err := json.Unmarshal([]byte(*message.Data), &dataStruct)
	if err != nil {
		return err
	}
	switch message.Resource {
	case "event":
		log.Debugf("Update message op %v Resource %v Data %v \n", message.Op, message.Resource, dataStruct)
		switch message.Op {
		case application.ADD:
			for _, d := range dataStruct {
				log.Debugf("Add message %v \n", d)
				err := e.addEvent(d.SourceTopic, d.SinkTopic)
				if err != nil {
					return err
				}
			}
		case application.DELETE:
			for _, d := range dataStruct {
				err := e.delEvent(d.SourceTopic)
				if err != nil {
					return err
				}
			}
		case application.UPDATE:
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
		case application.GET:
		default:
			log.Warnf("Unknown operation\n")
		}
	default:
		log.Warnf("Unknown resouces\n")
	}
	return nil
}

func (e eventRepeater) serve() error {
	if e.allowAll {
		e.addEvent(".", "")
	}
	for {
		select {
		case <-e.stop:
			return nil
		}
	}
}
