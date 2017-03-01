package broker

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/broker/client"
	"github.com/docker/infrakit/pkg/broker/server"
	"github.com/stretchr/testify/require"
)

func TestBrokerMultiSubscribers(t *testing.T) {

	broker := server.NewBroker()
	go http.ListenAndServe("localhost:3000", broker)

	received1 := make(chan interface{})
	received2 := make(chan interface{})

	topic1, _, err := client.Subscribe("http://localhost:3000/", "local", client.Options{})
	require.NoError(t, err)
	go func() {
		for {
			var val interface{}
			require.NoError(t, (<-topic1).Decode(&val))
			received1 <- val
		}
	}()

	topic2, _, err := client.Subscribe("http://localhost:3000/", "local/time", client.Options{})
	require.NoError(t, err)
	go func() {
		for {
			var val interface{}
			require.NoError(t, (<-topic2).Decode(&val))
			received2 <- val
		}
	}()

	go func() {
		for {
			<-time.After(1 * time.Millisecond)
			require.NoError(t, broker.Publish("local/time/now", time.Now().UnixNano()))
		}
	}()

	// Test a few rounds to make sure all subscribers get the same messages each round.
	for i := 0; i < 5; i++ {
		a := <-received1
		b := <-received2
		require.NotNil(t, a)
		require.NotNil(t, b)
		require.Equal(t, a, b)
	}

	broker.Stop()

}

func TestBrokerMultiSubscribersProducers(t *testing.T) {

	broker := server.NewBroker()
	go http.ListenAndServe("localhost:3001", broker)

	received1 := make(chan interface{})
	received2 := make(chan interface{})

	topic1, _, err := client.Subscribe("http://localhost:3001/", "local", client.Options{})
	require.NoError(t, err)
	go func() {
		for {
			var val interface{}
			require.NoError(t, (<-topic1).Decode(&val))
			received1 <- val
		}
	}()

	topic2, _, err := client.Subscribe("http://localhost:3001/", "local/time", client.Options{})
	require.NoError(t, err)
	go func() {
		for {
			var val interface{}
			require.NoError(t, (<-topic2).Decode(&val))
			received2 <- val
		}
	}()

	total := 10
	go func() {
		for i := 0; i < total; i++ {
			<-time.After(20 * time.Millisecond)
			require.NoError(t, broker.Publish("local/time/now", fmt.Sprintf("a:%d", time.Now().UnixNano())))
		}
	}()

	go func() {
		for i := 0; i < total; i++ {
			<-time.After(20 * time.Millisecond)
			require.NoError(t, broker.Publish("local/time/now", fmt.Sprintf("b:%d", time.Now().UnixNano())))
		}
	}()

	time.Sleep(3 * time.Second)
	count1, count2 := 0, 0
	// Test a few rounds to make sure all subscribers get the same messages each round.
	for i := 0; i < total; i++ {
		a := <-received1
		b := <-received2
		require.Equal(t, a, b)

		p := strings.Split(a.(string), ":")
		switch p[0] {
		case "a":
			count1++
		case "b":
			count2++
		}
	}

	require.Equal(t, count1, count2)

	broker.Stop()

}
