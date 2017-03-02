package client

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/broker/server"
	"github.com/stretchr/testify/require"
)

func TestBrokerMultiSubscribersEarlyDisconnects(t *testing.T) {

	broker := server.NewBroker()
	go http.ListenAndServe("localhost:6002", broker)

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

	topic1, _, err := Subscribe("http://localhost:6002/", "local", Options{})
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

	topic2, _, err := Subscribe("http://localhost:6002/?topic=/local/time", "", Options{})
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
	go http.ListenAndServe("localhost:6003", broker)

	received1 := make(chan event)
	received2 := make(chan event)

	topic1, _, err := Subscribe("http://localhost:6003/", "local", Options{})
	require.NoError(t, err)
	go func() {
		for {
			var val event
			require.NoError(t, (<-topic1).Decode(&val))
			received1 <- val
		}
	}()

	topic2, _, err := Subscribe("http://localhost:6003/", "local/instance1", Options{})
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
