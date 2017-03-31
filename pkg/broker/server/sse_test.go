package server

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/broker/client"
	"github.com/stretchr/testify/require"
)

func TestBrokerMultiSubscribers(t *testing.T) {
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
			var val interface{}
			require.NoError(t, (<-topic1).Decode(&val))
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
			<-time.After(20 * time.Millisecond)
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

func TestBrokerMultiSubscribersProducers(t *testing.T) {
	socketFile := tempSocket()
	socket := "unix://broker" + socketFile

	broker, err := ListenAndServeOnSocket(socketFile)
	require.NoError(t, err)

	received1 := make(chan interface{})
	received2 := make(chan interface{})

	opts := client.Options{SocketDir: filepath.Dir(socketFile)}

	sync := make(chan struct{})
	topic1, _, done1, err := client.Subscribe(socket, "local/", opts)
	require.NoError(t, err)
	go func() {
		<-sync
		for {
			var val interface{}
			require.NoError(t, (<-topic1).Decode(&val))
			received1 <- val
		}
	}()

	topic2, _, done2, err := client.Subscribe(socket+"/?topic=/local/time/", "", opts)
	require.NoError(t, err)
	go func() {
		<-sync
		for {
			var val interface{}
			require.NoError(t, (<-topic2).Decode(&val))
			received2 <- val
		}
	}()

	topic3, _, done3, err := client.Subscribe(socket, "cluster/time/", opts)
	require.NoError(t, err)
	go func() {
		<-sync
		if v, ok := <-topic3; ok {
			panic(v) // shouldn't receive a message here.
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

	close(sync)

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

	close(done1)
	close(done2)
	close(done3)
	broker.Stop()

}

func TestBrokerNoSubscribers(t *testing.T) {

	// Tests for stability of having lots of producers but no subscribers.
	socketFile := tempSocket()

	broker, err := ListenAndServeOnSocket(socketFile)
	require.NoError(t, err)

	total := 100
	var done sync.WaitGroup
	for _, i := range []int{1, 10, 10, 10, 5, 15, 17, 12, 20} {
		done.Add(1)
		delay := time.Duration(i)
		go func() {
			defer done.Done()
			for i := 0; i < total; i++ {
				broker.Publish("local/time/now", time.Now().UnixNano(), 10*time.Millisecond)
				<-time.After(delay * time.Millisecond)
			}
		}()
	}

	// We expect all the goroutines to complete and call done on the wait group.
	done.Wait()

	broker.Stop()

	// This is still possible even after the broker stopped.
	for i := 0; i < total; i++ {
		err = broker.Publish("local/time/now", time.Now().UnixNano())
		require.NoError(t, err)

	}
}
