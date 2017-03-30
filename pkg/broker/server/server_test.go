package server

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/broker/client"
	"github.com/stretchr/testify/require"
)

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return filepath.Join(dir, "broker")
}

func TestListenAndServeOnSocket(t *testing.T) {

	socketFile := tempSocket()
	socket := "unix://broker" + socketFile

	broker, err := ListenAndServeOnSocket(socketFile)
	require.NoError(t, err)

	received1 := make(chan interface{})
	received2 := make(chan interface{})

	opts := client.Options{SocketDir: filepath.Dir(socketFile)}
	topic1, _, stop1, err := client.Subscribe(socket, "local/", opts)
	require.NoError(t, err)
	go func() {
		for {
			any := <-topic1
			var val interface{}
			require.NoError(t, any.Decode(&val))
			received1 <- val
		}
	}()

	topic2, _, stop2, err := client.Subscribe(socket, "local/time/", opts)
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
			<-time.After(100 * time.Millisecond)

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

	close(stop1)
	close(stop2)
	broker.Stop()

}
