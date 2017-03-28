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
	"github.com/docker/infrakit/pkg/types"
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
	return &client{client: rpcClient, socketPath: rpcClient.Addr()}
}

type client struct {
	client     rpc_client.Client
	socketPath string
}

// List returns the nodes under a topic
func (c client) List(topic types.Path) ([]string, error) {
	req := ListRequest{Topic: topic}
	resp := ListResponse{}
	err := c.client.Call("Event.List", req, &resp)
	return resp.Nodes, err
}

// SubscribeOn returns the subscriber channel for the topic
func (c *client) SubscribeOn(topic types.Path) (<-chan *event.Event, error) {

	opts := broker.Options{SocketDir: filepath.Dir(c.socketPath), Path: rpc.URLEventsPrefix}
	url := fmt.Sprintf("unix://%s", filepath.Base(c.socketPath))

	topicStr := topic.String()
	if topic.Equal(types.PathFromString(".")) {
		topicStr = ""
	}

	log.Infoln("Connecting to broker url=", url, "topic=", topicStr, "opts=", opts)
	raw, errors, err := broker.Subscribe(url, topicStr, opts)
	if err != nil {
		return nil, err
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

	return typed, nil
}
