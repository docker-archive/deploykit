package main

import (
	"fmt"
	event_rpc "github.com/docker/infrakit/pkg/rpc/event"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/event"
	testing_event "github.com/docker/infrakit/pkg/testing/event"
	"github.com/docker/infrakit/pkg/types"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"
)

var MQTTTESTSERVER string = "tcp://test.mosquitto.org:1883"

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}
	return filepath.Join(dir, "app-impl-test")
}
func runEvent(startPub chan struct{}) (*string, rpc_server.Stoppable, error) {
	socketPath := tempSocket()
	events := 5
	publishChan0 := make(chan chan<- *event.Event)
	go func() {
		publish := <-publishChan0
		defer close(publish)
		<-startPub
		// here we have the channel and ready to go
		for i := 0; i < events; i++ {
			<-time.After(50 * time.Millisecond)
			fmt.Printf("publish event%d\n", i)
			publish <- event.Event{
				Topic: types.PathFromString("instance/create"),
				ID:    fmt.Sprintf("host-%d", i),
			}.Init().WithDataMust([]int{1, 2}).Now()
		}
	}()
	m := map[string]interface{}{}
	types.Put(types.PathFromString("instance/create"), "instance-create", m)
	plugin0 := &testing_event.Plugin{
		DoList: func(topic types.Path) ([]string, error) {
			return types.List(topic, m), nil
		},
		Publisher: &testing_event.Publisher{
			DoPublishOn: func(c chan<- *event.Event) {
				publishChan0 <- c
				close(publishChan0)
			},
		},
	}
	var impl rpc_server.VersionedInterface = event_rpc.PluginServerWithTypes(
		map[string]event.Plugin{
			"iktest": plugin0,
		})
	server, err := rpc_server.StartPluginAtPath(socketPath, impl)
	if err != nil {
		return nil, nil, err
	}
	return &socketPath, server, nil
}

func runSub(msgch chan MQTT.Message) (MQTT.Client, error) {
	opts := MQTT.NewClientOptions().AddBroker(MQTTTESTSERVER)
	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}
	subToken := client.Subscribe(
		"iktest/instance/create",
		0,
		func(client MQTT.Client, msg MQTT.Message) {
			msgch <- msg
		})
	fmt.Printf("mqtt substart")
	if subToken.Wait() && subToken.Error() != nil {
		return nil, subToken.Error()
	}
	return client, nil
}

func TestIntegration(t *testing.T) {
	startPub := make(chan struct{})
	socketPath, erpcsrv, err := runEvent(startPub)
	defer erpcsrv.Stop()
	require.NoError(t, err)
	mqsubch := make(chan MQTT.Message)
	mqttClient, err := runSub(mqsubch)
	require.NoError(t, err)
	defer mqttClient.Disconnect(250)
	app := NewEventRepeater(*socketPath, MQTTTESTSERVER, "mqtt", true)
	defer app.(*eventRepeater).Stop()
	close(startPub)
	var subEvent int = 0
loop:
	for {
		select {
		case <-time.After(500 * time.Millisecond):
			break loop
		case sub := <-mqsubch:
			subany := types.AnyBytes(sub.Payload())
			subevent := event.Event{}
			err := subany.Decode(&subevent)
			require.NoError(t, err)
			require.Equal(t, subevent.ID, fmt.Sprintf("host-%d", subEvent))
			subEvent++
		}
	}
}
