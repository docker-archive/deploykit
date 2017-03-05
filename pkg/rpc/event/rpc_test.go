package event

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/event"
	testing_event "github.com/docker/infrakit/pkg/testing/event"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return filepath.Join(dir, "event-impl-test")
}

func must(p event.Plugin, err error) event.Plugin {
	if err != nil {
		panic(err)
	}
	return p
}

func mustNotErr(err error) {
	if err != nil {
		panic(err)
	}
}

func first(a, b interface{}) interface{} {
	return a
}

func firstAny(a, b interface{}) *types.Any {
	v := first(a, b)
	return v.(*types.Any)
}

func second(a, b interface{}) interface{} {
	return b
}

func topics(a string, b ...string) []event.Topic {
	out := []event.Topic{event.NewTopic(a)}
	for _, t := range b {
		out = append(out, event.NewTopic(t))
	}
	return out
}

func TestEventMultiPlugin(t *testing.T) {
	socketPath := tempSocket()

	calledTopics0 := make(chan struct{})
	calledTopics1 := make(chan struct{})
	calledTopics2 := make(chan struct{})
	calledTopics3 := make(chan struct{})

	list := []event.Topic{"instance/create", "instance/delete"}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]event.Plugin{
			"compute": &testing_event.Plugin{
				Publisher: (*testing_event.Publisher)(nil),
				DoTopics: func() ([]event.Topic, error) {
					close(calledTopics1)
					return list, nil
				},
			},
			"storage": &testing_event.Plugin{
				Publisher: (*testing_event.Publisher)(nil),
				DoTopics: func() ([]event.Topic, error) {
					close(calledTopics2)
					return list, nil
				},
			},
			"bad": &testing_event.Plugin{
				Publisher: (*testing_event.Publisher)(nil),
				DoTopics: func() ([]event.Topic, error) {
					close(calledTopics3)
					return nil, errors.New("error")
				},
			},
		}).WithBase(
		&testing_event.Plugin{
			Publisher: (*testing_event.Publisher)(nil),
			DoTopics: func() ([]event.Topic, error) {
				close(calledTopics0)
				return list, nil
			},
		}))
	require.NoError(t, err)

	require.Equal(t, topics(
		"compute/instance/create",
		"compute/instance/delete",
		"instance/create",
		"instance/delete",
		"storage/instance/create",
		"storage/instance/delete",
	), first(must(NewClient(socketPath)).Topics()))

	<-calledTopics0
	<-calledTopics1
	<-calledTopics2
	<-calledTopics3

	server.Stop()
}

func TestEventPluginPublishSubscribe(t *testing.T) {
	socketPath := tempSocket()

	calledTopics0 := make(chan struct{})
	calledTopics1 := make(chan struct{})
	calledPublisher0 := make(chan struct{})
	calledPublisher1 := make(chan struct{})

	list := []event.Topic{"instance/create", "instance/delete"}

	sync := make(chan struct{}) // for synchronizing all the goroutines and don't publish until ready to record test

	events := 5
	publishChan0 := make(chan chan<- *event.Event)
	go func() {
		publish := <-publishChan0
		defer close(publish)
		<-sync

		// here we have the channel and ready to go
		for i := 0; i < events; i++ {

			<-time.After(50 * time.Millisecond)

			publish <- event.Event{
				Topic: event.Topic("instance/create"),
				ID:    fmt.Sprintf("host-%d", i),
			}.Init().WithDataMust([]int{1, 2}).Now()
		}
	}()

	publishChan1 := make(chan chan<- *event.Event)
	go func() {
		publish := <-publishChan1
		defer close(publish)
		<-sync

		// here we have the channel and ready to go
		for i := 0; i < events; i++ {

			<-time.After(50 * time.Millisecond)

			publish <- event.Event{
				Topic: event.Topic("instance/create"),
				ID:    fmt.Sprintf("disk-%d", i),
			}.Init().WithDataMust([]string{"foo", "bar"}).Now()
		}
	}()

	plugin0 := &testing_event.Plugin{
		DoTopics: func() ([]event.Topic, error) {
			close(calledTopics0)
			return list, nil
		},
		Publisher: &testing_event.Publisher{
			DoPublishOn: func(c chan<- *event.Event) {
				publishChan0 <- c
				close(calledPublisher0)
				close(publishChan0)
			},
		},
	}

	plugin1 := &testing_event.Plugin{
		DoTopics: func() ([]event.Topic, error) {
			close(calledTopics1)
			return list, nil
		},
		Publisher: &testing_event.Publisher{
			DoPublishOn: func(c chan<- *event.Event) {
				publishChan1 <- c
				close(calledPublisher1)
				close(publishChan1)
			},
		},
	}

	// check that they are also publishers
	var pub1, pub2 event.Publisher = plugin0, plugin1
	require.NotNil(t, pub1)
	require.NotNil(t, pub2)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]event.Plugin{
			"compute": plugin0,
			"storage": plugin1,
		}))
	require.NoError(t, err)

	require.Equal(t, topics(
		"compute/instance/create",
		"compute/instance/delete",
		"storage/instance/create",
		"storage/instance/delete",
	), first(must(NewClient(socketPath)).Topics()))

	<-calledTopics0
	<-calledTopics1

	<-calledPublisher0
	<-calledPublisher1

	client := must(NewClient(socketPath)).(event.Subscriber)
	require.NotNil(t, client)

	compute, err := client.SubscribeOn(event.NewTopic("compute"))
	require.NoError(t, err)

	storage, err := client.SubscribeOn(event.NewTopic("storage"))
	require.NoError(t, err)

	all, err := client.SubscribeOn(event.NewTopic("/"))
	require.NoError(t, err)

	computeEvents := []*event.Event{}
	storageEvents := []*event.Event{}
	allEvents := []*event.Event{}

	close(sync) // start publishing the messages

loop:
	for {
		select {

		case <-time.After(500 * time.Millisecond):
			break loop

		case m := <-all:
			allEvents = append(allEvents, m)

		case m := <-compute:
			computeEvents = append(computeEvents, m)

		case m := <-storage:
			storageEvents = append(storageEvents, m)
		}
	}

	server.Stop()

	require.Equal(t, len(allEvents), len(computeEvents)+len(storageEvents))
	require.Equal(t, events, len(computeEvents))
	require.Equal(t, events, len(storageEvents))

}
