package event

import (
	"fmt"
	"path"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	broker "github.com/docker/infrakit/pkg/broker/client"
	"github.com/docker/infrakit/pkg/rpc"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/types"
)

// NewClient returns a plugin interface implementation connected to a remote plugin
func NewClient(address string) (event.Plugin, error) {
	rpcClient, err := rpc_client.New(address, event.InterfaceSpec)
	if err != nil {
		return nil, err
	}
	return &client{client: rpcClient, address: address}, nil
}

// Adapt converts a rpc client to a event plugin object
func Adapt(rpcClient rpc_client.Client) event.Plugin {
	return &client{client: rpcClient, address: rpcClient.Addr()}
}

type client struct {
	client  rpc_client.Client
	address string
}

// List returns the nodes under a topic
func (c client) List(topic types.Path) ([]string, error) {
	req := ListRequest{Topic: topic}
	resp := ListResponse{}
	err := c.client.Call("Event.List", req, &resp)
	return resp.Nodes, err
}

// SubscribeOn returns the subscriber channel for the topic
func (c *client) SubscribeOn(topic types.Path) (<-chan *event.Event, chan<- struct{}, error) {

	opts := broker.Options{SocketDir: path.Dir(c.address), Path: rpc.URLEventsPrefix}

	url := fmt.Sprintf("unix://%s", path.Base(c.address))

	// check to see the address isn't a url
	if strings.Contains(c.address, "://") {
		url = c.address
		log.Infoln("Connecting to network event stream:", url, "opts", opts)
	}

	topicStr := topic.String()
	if topic.Equal(types.PathFromString(".")) {
		topicStr = ""
	}

	log.Infoln("Connecting to broker url=", url, "topic=", topicStr, "opts=", opts)
	raw, errors, done, err := broker.Subscribe(url, topicStr, opts)
	if err != nil {
		return nil, nil, err
	}

	typed := make(chan *event.Event)

	go func() {
		topicCopy := topic
		defer close(typed)
		for {
			select {
			case err, ok := <-errors:
				if ok {
					typed <- event.Event{
						Topic:    topicCopy,
						Type:     event.TypeError,
						Received: time.Now(),
					}.Init().WithError(err)
				}
			case any, ok := <-raw:
				if !ok {
					return
				}
				typed <- new(event.Event).FromAny(any).ReceivedNow()
			}

		}
	}()

	return typed, done, nil
}
