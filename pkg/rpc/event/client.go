package event

import (
	"fmt"
	"path/filepath"
	"time"

	log "github.com/Sirupsen/logrus"
	broker "github.com/docker/infrakit/pkg/broker/client"
	"github.com/docker/infrakit/pkg/rpc"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi/event"
)

// NewClient returns a plugin interface implementation connected to a remote plugin
func NewClient(socketPath string) (event.Plugin, error) {
	rpcClient, err := rpc_client.New(socketPath, event.InterfaceSpec)
	if err != nil {
		return nil, err
	}
	return &client{client: rpcClient, socketPath: socketPath}, nil
}

// Adapt converts a rpc client to a event plugin object
func Adapt(rpcClient rpc_client.Client) event.Plugin {
	return &client{client: rpcClient}
}

type client struct {
	client     rpc_client.Client
	socketPath string
}

// Topics return a set of topics
func (c client) Topics() ([]event.Topic, error) {
	req := TopicsRequest{}
	resp := TopicsResponse{}
	err := c.client.Call("Event.Topics", req, &resp)
	return resp.Topics, err
}

// SubscribeOn returns the subscriber channel for the topic
func (c *client) SubscribeOn(t event.Topic) (<-chan *event.Event, error) {

	opts := broker.Options{SocketDir: filepath.Dir(c.socketPath), Path: rpc.URLEventsPrefix}
	url := fmt.Sprintf("unix://%s", filepath.Base(c.socketPath))

	log.Infoln("Connect to broker url=", url, "topic=", t.String(), "opts=", opts)

	raw, errors, err := broker.Subscribe(url, t.String(), opts)
	if err != nil {
		return nil, err
	}

	typed := make(chan *event.Event)

	go func() {
		topic := t
		defer close(typed)
		for {
			select {
			case err, ok := <-errors:
				if ok {
					typed <- event.Event{
						Topic:    topic,
						Type:     event.TypeError,
						Received: time.Now(),
					}.Init().WithError(err)
				}
			case any, ok := <-raw:
				if !ok {
					return
				} else {
					typed <- new(event.Event).FromAny(any).ReceivedNow()
				}
			}

		}
	}()

	return typed, nil
}
