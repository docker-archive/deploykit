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

	topic2, _, err := client.Subscribe("http://localhost:3001/?topic=/local/time", "", client.Options{})
	require.NoError(t, err)
	go func() {
		for {
			var val interface{}
			require.NoError(t, (<-topic2).Decode(&val))
			received2 <- val
		}
	}()

	topic3, _, err := client.Subscribe("http://localhost:3001", "cluster/time", client.Options{})
	require.NoError(t, err)
	go func() {
		panic(<-topic3)
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
	require.True(t, count1 > 0)
	require.Equal(t, count1, count2)

	broker.Stop()

}

func TestBrokerMultiSubscribersEarlyDisconnects(t *testing.T) {

	broker := server.NewBroker()
	go http.ListenAndServe("localhost:3002", broker)

	// Start sending events right away, continously
	go func() {
		tick := 0
		for {
			<-time.After(100 * time.Millisecond)
			require.NoError(t, broker.Publish("local/time/tick", tick))
			tick++

			if tick > 30 {
				broker.Stop()
				return
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)

	received1 := make(chan interface{})
	received2 := make(chan interface{})

	topic1, _, err := client.Subscribe("http://localhost:3002/", "local", client.Options{})
	require.NoError(t, err)
	go func() {
		// This subscriber will leave after receiving 5 messages
		for {
			var val int
			require.NoError(t, (<-topic1).Decode(&val))
			received1 <- val

			if val > 10 {
				close(received1)
				return
			}
		}
	}()

	topic2, _, err := client.Subscribe("http://localhost:3002/?topic=/local/time", "", client.Options{})
	require.NoError(t, err)
	go func() {
		for {
			var val int
			require.NoError(t, (<-topic2).Decode(&val))
			received2 <- val

			if val > 20 {
				close(received2)
				return
			}
		}
	}()

	values1 := []interface{}{}
	values2 := []interface{}{}

	for v := range received1 {
		if v == nil {
			break
		}
		values1 = append(values1, v)
	}
	for v := range received2 {
		if v == nil {
			break
		}
		values2 = append(values2, v)
	}

	require.Equal(t, 10, len(values1))
	require.Equal(t, 20, len(values2))
}

func TestBrokerMultiSubscriberCustomObject(t *testing.T) {

	type event struct {
		Time    int64
		Message string
	}

	broker := server.NewBroker()
	go http.ListenAndServe("localhost:3003", broker)

	received1 := make(chan event)
	received2 := make(chan event)

	topic1, _, err := client.Subscribe("http://localhost:3003/", "local", client.Options{})
	require.NoError(t, err)
	go func() {
		for {
			var val event
			require.NoError(t, (<-topic1).Decode(&val))
			received1 <- val
		}
	}()

	topic2, _, err := client.Subscribe("http://localhost:3003/", "local/instance1", client.Options{})
	require.NoError(t, err)
	go func() {
		for {
			var val event
			require.NoError(t, (<-topic2).Decode(&val))
			received2 <- val
		}
	}()

	go func() {
		for {
			<-time.After(10 * time.Millisecond)

			now := time.Now()
			evt := event{Time: now.UnixNano(), Message: fmt.Sprintf("Now is %v", now)}
			require.NoError(t, broker.Publish("remote/instance1", evt))
			require.NoError(t, broker.Publish("local/instance1", evt))
		}
	}()

	// Test a few rounds to make sure all subscribers get the same messages each round.
	for i := 0; i < 5; i++ {
		a := <-received1
		b := <-received2
		require.Equal(t, a, b)
	}

	broker.Stop()

}
