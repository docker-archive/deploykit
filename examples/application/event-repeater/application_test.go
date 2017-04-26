package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	event_rpc "github.com/docker/infrakit/pkg/rpc/event"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/application"
	"github.com/docker/infrakit/pkg/spi/event"
	testing_event "github.com/docker/infrakit/pkg/testing/event"
	"github.com/docker/infrakit/pkg/types"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

var MQTTTESTSERVER = "tcp://iot.eclipse.org:1883"
var EVENTNUM = 5

func TestUnitUpdate(t *testing.T) {
	socketPath := tempSocket()
	stub := func() interface{} { return nil }
	m := map[string]interface{}{}
	types.Put(types.PathFromString("test/instance1"), stub, m)
	types.Put(types.PathFromString("test/instance2"), stub, m)
	plugin := &testing_event.Plugin{
		Publisher: (*testing_event.Publisher)(nil),
		DoList: func(topic types.Path) ([]string, error) {
			return types.List(topic, m), nil
		},
	}
	var impl rpc_server.VersionedInterface = event_rpc.PluginServerWithTypes(
		map[string]event.Plugin{
			"test": plugin,
		})
	server, err := rpc_server.StartPluginAtPath(socketPath, impl)
	require.NoError(t, err)
	defer server.Stop()

	//Test ADD operation
	require.NoError(t, err)
	e := NewEventRepeater(socketPath, "", "stderr", false).(*eventRepeater)
	mes := &application.Message{
		Op:       application.ADD,
		Resource: "event",
		Data:     types.AnyString("[{\"sourcetopic\":\"test/instance1\",\"sinktopic\":\"test/sink/instance1\"},{\"sourcetopic\":\"test/instance2\",\"sinktopic\":\"test/sink/instance2\"}]"),
	}
	err = e.Update(mes)
	require.NoError(t, err)
	require.Equal(t, 2, len(e.Events))
	require.Equal(t, "test/sink/instance1", e.Events["test/instance1"].SinkTopic)
	require.Equal(t, "test/sink/instance2", e.Events["test/instance2"].SinkTopic)

	//Test DELETE operation
	mes = &application.Message{
		Op:       application.DELETE,
		Resource: "event",
		Data:     types.AnyString("[{\"sourcetopic\":\"test/instance2\",\"sinktopic\":\"\"}]"),
	}
	err = e.Update(mes)
	require.NoError(t, err)
	require.Equal(t, 1, len(e.Events))
	require.Equal(t, "test/sink/instance1", e.Events["test/instance1"].SinkTopic)
	_, ok := e.Events["test/instance2"]
	require.Equal(t, false, ok)

	//Test UPDATE operation
	mes = &application.Message{
		Op:       application.UPDATE,
		Resource: "event",
		Data:     types.AnyString("[{\"sourcetopic\":\"test/instance1\",\"sinktopic\":\"test/event/instance1\"}]"),
	}
	err = e.Update(mes)
	require.NoError(t, err)
	require.Equal(t, "test/event/instance1", e.Events["test/instance1"].SinkTopic)
}

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}
	return filepath.Join(dir, "app-impl-test")
}
func runEvent(startPub chan struct{}, tPrefix string) (string, rpc_server.Stoppable, error) {
	socketPath := tempSocket()
	publishChan0 := make(chan chan<- *event.Event)
	go func() {
		publish := <-publishChan0
		defer close(publish)
		<-startPub
		// here we have the channel and ready to go
		for i := 0; i < EVENTNUM; i++ {
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
		<-startPub
		// here we have the channel and ready to go
		for i := 0; i < EVENTNUM; i++ {
			<-time.After(50 * time.Millisecond)
			publish <- event.Event{
				Topic: types.PathFromString("instance/create"),
				ID:    fmt.Sprintf("disk-%d", i),
			}.Init().WithDataMust([]string{"foo", "bar"}).Now()
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
	plugin1 := &testing_event.Plugin{
		DoList: func(topic types.Path) ([]string, error) {
			return types.List(topic, m), nil
		},
		Publisher: &testing_event.Publisher{
			DoPublishOn: func(c chan<- *event.Event) {
				publishChan1 <- c
				close(publishChan1)
			},
		},
	}
	var impl rpc_server.VersionedInterface = event_rpc.PluginServerWithTypes(
		map[string]event.Plugin{
			tPrefix + "-compute": plugin0,
			tPrefix + "-storage": plugin1,
		})
	server, err := rpc_server.StartPluginAtPath(socketPath, impl)
	if err != nil {
		return "", nil, err
	}
	return socketPath, server, nil
}

func runSub(msgch chan MQTT.Message, tPrefix string) (MQTT.Client, error) {
	opts := MQTT.NewClientOptions().AddBroker(MQTTTESTSERVER)
	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}
	subToken := client.Subscribe(
		tPrefix+"/instance/create",
		0,
		func(client MQTT.Client, msg MQTT.Message) {
			msgch <- msg
		})
	if subToken.Wait() && subToken.Error() != nil {
		return nil, subToken.Error()
	}
	return client, nil
}

func TestIntegrationAllowAll(t *testing.T) {
	var n uint64
	binary.Read(rand.Reader, binary.LittleEndian, &n)
	randString := strconv.FormatUint(n, 36)
	topicPrefix := "ifktest-" + randString
	startPub := make(chan struct{})
	socketPath, erpcsrv, err := runEvent(startPub, topicPrefix)
	defer erpcsrv.Stop()
	require.NoError(t, err)
	mqsubch0 := make(chan MQTT.Message)
	mqttClient0, err := runSub(mqsubch0, topicPrefix+"-compute")
	require.NoError(t, err)
	mqsubch1 := make(chan MQTT.Message)
	mqttClient1, err := runSub(mqsubch1, topicPrefix+"-storage")
	require.NoError(t, err)
	defer mqttClient0.Disconnect(250)
	defer mqttClient1.Disconnect(250)
	app := NewEventRepeater(socketPath, MQTTTESTSERVER, "mqtt", true)
	defer app.(*eventRepeater).Stop()
	close(startPub)
	var subEvent0 int
	var subEvent1 int
loop:
	for {
		select {
		case <-time.After(500 * time.Millisecond):
			break loop
		case sub0 := <-mqsubch0:
			subany := types.AnyBytes(sub0.Payload())
			subevent := event.Event{}
			err := subany.Decode(&subevent)
			require.NoError(t, err)
			require.Equal(t, fmt.Sprintf("host-%d", subEvent0), subevent.ID)
			subEvent0++
		case sub1 := <-mqsubch1:
			subany := types.AnyBytes(sub1.Payload())
			subevent := event.Event{}
			err := subany.Decode(&subevent)
			require.NoError(t, err)
			require.Equal(t, fmt.Sprintf("disk-%d", subEvent1), subevent.ID)
			subEvent1++

		}
	}
	require.Equal(t, EVENTNUM, subEvent0)
	require.Equal(t, EVENTNUM, subEvent1)
}

func TestIntegrationDenyAll(t *testing.T) {
	var n uint64
	binary.Read(rand.Reader, binary.LittleEndian, &n)
	randString := strconv.FormatUint(n, 36)
	topicPrefix := "ifktest-" + randString
	startPub := make(chan struct{})
	socketPath, erpcsrv, err := runEvent(startPub, topicPrefix)
	defer erpcsrv.Stop()
	require.NoError(t, err)
	mqsubch0 := make(chan MQTT.Message)
	mqttClient0, err := runSub(mqsubch0, topicPrefix+"-compute")
	require.NoError(t, err)
	mqsubch1 := make(chan MQTT.Message)
	mqttClient1, err := runSub(mqsubch1, topicPrefix+"-storage")
	require.NoError(t, err)
	defer mqttClient0.Disconnect(250)
	defer mqttClient1.Disconnect(250)
	app := NewEventRepeater(socketPath, MQTTTESTSERVER, "mqtt", false)
	defer app.(*eventRepeater).Stop()
	m := &application.Message{
		Op:       application.ADD,
		Resource: "event",
		Data:     types.AnyString("[{\"sourcetopic\":\"" + topicPrefix + "-compute/instance/create\",\"sinktopic\":\"\"}]"),
	}
	err = app.Update(m)
	require.NoError(t, err)
	close(startPub)
	var subEvent0 int
	var subEvent1 int
loop:
	for {
		select {
		case <-time.After(500 * time.Millisecond):
			break loop
		case sub0 := <-mqsubch0:
			subany := types.AnyBytes(sub0.Payload())
			subevent := event.Event{}
			err := subany.Decode(&subevent)
			require.NoError(t, err)
			require.Equal(t, fmt.Sprintf("host-%d", subEvent0), subevent.ID)
			subEvent0++
		case sub1 := <-mqsubch1:
			subany := types.AnyBytes(sub1.Payload())
			subevent := event.Event{}
			err := subany.Decode(&subevent)
			require.NoError(t, err)
			require.Equal(t, fmt.Sprintf("disk-%d", subEvent1), subevent.ID)
			subEvent1++
		}
	}
	require.Equal(t, EVENTNUM, subEvent0)
	require.Equal(t, 0, subEvent1)
}
