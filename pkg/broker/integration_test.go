package broker

import (
	"net/http"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/broker/client"
	"github.com/docker/infrakit/pkg/broker/server"
	"github.com/stretchr/testify/require"
)

func TestBroker(t *testing.T) {

	broker := server.NewBroker()
	go http.ListenAndServe("localhost:3000", broker)

	received1 := make(chan interface{})
	received2 := make(chan interface{})

	topic1, _, err := client.Subscribe("http://localhost:3000/", "local", nil)
	require.NoError(t, err)
	go func() {
		for {
			m := <-topic1
			received1 <- m
		}
	}()

	topic2, _, err := client.Subscribe("http://localhost:3000/", "local/time", nil)
	require.NoError(t, err)
	go func() {
		for {
			m := <-topic2
			received2 <- m
		}
	}()

	go func() {
		for {
			<-time.After(10 * time.Millisecond)
			require.NoError(t, broker.Send("local/time/now", time.Now().Unix()))
		}
	}()

	// Test a few rounds to make sure all subscribers get the same messages each round.
	for i := 0; i < 5; i++ {
		require.Equal(t, <-received1, <-received2)
	}

	broker.Stop()

}
