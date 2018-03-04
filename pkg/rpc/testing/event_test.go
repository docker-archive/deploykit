package testing

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	rpc_event "github.com/docker/infrakit/pkg/rpc/event"
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

func first(a, b interface{}) interface{} {
	return a
}

func TestEventMultiPlugin(t *testing.T) {
	socketPath := tempSocket()

	stub := func() interface{} { return nil } // just a placeholder because the reflect api doesn't like nil.

	m := map[string]interface{}{}
	types.Put(types.PathFromString("instance/create"), stub, m)
	types.Put(types.PathFromString("instance/delete"), stub, m)

	server, err := rpc_server.StartPluginAtPath(socketPath, rpc_event.PluginServerWithNames(
		func() (map[string]event.Plugin, error) {
			return map[string]event.Plugin{
				"compute": &testing_event.Plugin{
					Publisher: (*testing_event.Publisher)(nil),
					DoList: func(topic types.Path) ([]string, error) {
						return types.List(topic, m), nil
					},
				},
				"storage": &testing_event.Plugin{
					Publisher: (*testing_event.Publisher)(nil),
					DoList: func(topic types.Path) ([]string, error) {
						return types.List(topic, m), nil
					},
				},
				"device": &testing_event.Plugin{
					Publisher: (*testing_event.Publisher)(nil),
					DoList: func(topic types.Path) ([]string, error) {
						return nil, nil
					},
				},
			}, nil
		}).WithBase(
		&testing_event.Plugin{
			Publisher: (*testing_event.Publisher)(nil),
			DoList: func(topic types.Path) ([]string, error) {
				return types.List(topic, m), nil
			},
		}))
	require.NoError(t, err)

	require.Equal(t, []string{"compute", "device", "instance", "storage"},
		first(must(rpc_event.NewClient(socketPath)).List(types.PathFromString("."))))

	require.Equal(t, []string{"instance"},
		first(must(rpc_event.NewClient(socketPath)).List(types.PathFromString("compute"))))

	require.Equal(t, []string{"create", "delete"},
		first(must(rpc_event.NewClient(socketPath)).List(types.PathFromString("storage/instance"))))

	require.Equal(t, []string{"create", "delete"},
		first(must(rpc_event.NewClient(socketPath)).List(types.PathFromString("compute/instance"))))

	require.Equal(t, []string(nil),
		first(must(rpc_event.NewClient(socketPath)).List(types.PathFromString("device"))))

	server.Stop()
}

func TestEventPluginPublishSubscribe(t *testing.T) {
	socketPath := tempSocket()

	calledPublisher0 := make(chan struct{})
	calledPublisher1 := make(chan struct{})

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
				Topic: types.PathFromString("instance/create"),
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
				Topic: types.PathFromString("instance/create"),
				ID:    fmt.Sprintf("disk-%d", i),
			}.Init().WithDataMust([]string{"foo", "bar"}).Now()
		}
	}()

	m := map[string]interface{}{}
	types.Put(types.PathFromString("instance/create"), "instance-create", m)
	types.Put(types.PathFromString("instance/delete"), "instance-delete", m)

	plugin0 := &testing_event.Plugin{
		DoList: func(topic types.Path) ([]string, error) {
			return types.List(topic, m), nil
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
		DoList: func(topic types.Path) ([]string, error) {
			return types.List(topic, m), nil
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

	var impl rpc_server.VersionedInterface = rpc_event.PluginServerWithNames(
		func() (map[string]event.Plugin, error) {
			return map[string]event.Plugin{
				"compute": plugin0,
				"storage": plugin1,
			}, nil
		})

	server, err := rpc_server.StartPluginAtPath(socketPath, impl)
	require.NoError(t, err)

	<-calledPublisher0
	<-calledPublisher1

	validator, is := impl.(event.Validator)
	require.True(t, is)

	err = validator.Validate(types.PathFromString(""))
	require.NoError(t, err)

	err = validator.Validate(types.PathFromString("compute"))
	require.NoError(t, err)

	err = validator.Validate(types.PathFromString("compute/"))
	require.NoError(t, err)

	err = validator.Validate(types.PathFromString("storage"))
	require.NoError(t, err)

	err = validator.Validate(types.PathFromString("storage/"))
	require.NoError(t, err)

	err = validator.Validate(types.PathFromString("storage/instance"))
	require.NoError(t, err)

	err = validator.Validate(types.PathFromString("storage/instance/create"))
	require.NoError(t, err)

	err = validator.Validate(types.PathFromString("storage/instance/c"))
	require.Error(t, err)

	err = validator.Validate(types.PathFromString("stor"))
	require.Error(t, err)

	err = validator.Validate(types.PathFromString("computer"))
	require.Error(t, err)

	client := must(rpc_event.NewClient(socketPath)).(event.Subscriber)
	require.NotNil(t, client)

	compute, doneCompute, err := client.SubscribeOn(types.PathFromString("compute/"))
	require.NoError(t, err)

	storage, doneStorage, err := client.SubscribeOn(types.PathFromString("storage/"))
	require.NoError(t, err)

	all, doneAll, err := client.SubscribeOn(types.PathFromString("."))
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

	close(doneCompute)
	close(doneStorage)
	close(doneAll)

	server.Stop()

	require.Equal(t, events, len(computeEvents))
	require.Equal(t, events, len(storageEvents))
	require.Equal(t, len(allEvents), len(computeEvents)+len(storageEvents))
}
