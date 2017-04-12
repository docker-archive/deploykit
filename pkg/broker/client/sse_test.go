package client

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/broker/server"
	"github.com/stretchr/testify/require"
)

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return filepath.Join(dir, "broker")
}

func TestBrokerMultiSubscribersEarlyDisconnects(t *testing.T) {

	socketFile := tempSocket()
	socket := "unix://broker" + socketFile

	broker, err := server.ListenAndServeOnSocket(socketFile)
	require.NoError(t, err)

	sync := make(chan struct{})
	// Start sending events right away, continously
	go func() {
		<-sync
		tick := 1
		for {
			<-time.After(100 * time.Millisecond)
			require.NoError(t, broker.Publish("local/time/tick", tick))
			tick++

			if tick == 30 {
				broker.Stop()
				return
			}
		}
	}()

	received0 := make(chan interface{})
	received1 := make(chan interface{})
	received2 := make(chan interface{})

	opts := Options{SocketDir: filepath.Dir(socketFile)}

	// Note that two clients are subscribing to the same topic:

	topic0, _, done0, err := Subscribe(socket, "local/", opts)
	require.NoError(t, err)
	go func() {
		<-sync
		// This subscriber will leave after receiving 5 messages
		for {
			var val int
			require.NoError(t, (<-topic0).Decode(&val))
			received0 <- val

			if val == 10 {
				close(received0)
				return
			}
		}
	}()
	topic1, _, done1, err := Subscribe(socket, "local/", opts)
	require.NoError(t, err)
	go func() {
		<-sync
		// This subscriber will leave after receiving 5 messages
		for {
			var val int
			require.NoError(t, (<-topic1).Decode(&val))
			received1 <- val

			if val == 10 {
				close(received1)
				return
			}
		}
	}()

	topic2, _, done2, err := Subscribe(socket+"/?topic=/local/time/", "", opts)
	require.NoError(t, err)
	go func() {
		<-sync
		for {
			var val int
			require.NoError(t, (<-topic2).Decode(&val))
			received2 <- val

			if val == 20 {
				close(received2)
				return
			}
		}
	}()

	close(sync)

	values0 := []interface{}{}
	values1 := []interface{}{}
	values2 := []interface{}{}

	for v := range received0 {
		if v == nil {
			break
		}
		values0 = append(values0, v)
	}
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

	require.Equal(t, 20, len(values2))
	require.Equal(t, 10, len(values1))
	require.Equal(t, 10, len(values0))

	close(done0)
	close(done1)
	close(done2)
}

func TestBrokerMultiSubscriberCustomObject(t *testing.T) {

	type event struct {
		Time    int64
		Message string
	}

	socketFile := tempSocket()
	socket := "unix://broker" + socketFile

	broker, err := server.ListenAndServeOnSocket(socketFile)
	require.NoError(t, err)

	received1 := make(chan event)
	received2 := make(chan event)

	opts := Options{SocketDir: filepath.Dir(socketFile)}

	topic1, errs1, done1, err := Subscribe(socket, "local/", opts)
	require.NoError(t, err)
	go func() {
		for {
			select {

			case e := <-errs1:
				panic(e)
			case m, ok := <-topic1:
				if ok {
					var val event
					require.NoError(t, m.Decode(&val))
					received1 <- val
				} else {
					close(received1)
				}
			}
		}
	}()

	topic2, errs2, done2, err := Subscribe(socket, "local/instance1", opts)
	require.NoError(t, err)
	go func() {
		for {
			select {

			case e := <-errs2:
				panic(e)
			case m, ok := <-topic2:
				if ok {
					var val event
					require.NoError(t, m.Decode(&val))
					received2 <- val
				} else {
					close(received2)
				}
			}
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
		require.NotNil(t, a)
		require.NotEqual(t, "", a.Message)
		require.Equal(t, a, b)
	}

	close(done1)
	close(done2)

	broker.Stop()

}

func TestBrokerMultiSubscriberPartialMatchTopic(t *testing.T) {
	type event struct {
		Time    int64
		Message string
	}

	socketFile := tempSocket()
	socket := "unix://broker" + socketFile

	broker, err := server.ListenAndServeOnSocket(socketFile)
	require.NoError(t, err)

	received1 := make(chan event)
	received2 := make(chan event)

	opts := Options{SocketDir: filepath.Dir(socketFile)}

	start := make(chan struct{})

	go func() {
		<-start

		topic1, errs1, done1, err := Subscribe(socket, "local/instance", opts)
		require.NoError(t, err)

		defer close(done1)

		for {
			select {
			case e := <-errs1:
				t.Log("!!!!!!!!!!!!!!!!! FLAKY TEST !!!!!!!!!!!!", e)
			case m, ok := <-topic1:
				if ok {
					var val event
					require.NoError(t, m.Decode(&val))
					received1 <- val
				} else {
					close(received1)
				}
			}
		}
	}()

	go func() {
		<-start

		topic2, errs2, done2, err := Subscribe(socket, "local/instancetest", opts)
		require.NoError(t, err)

		defer close(done2)

		for {
			select {
			case e := <-errs2:
				t.Log("!!!!!!!!!!!!!!!!! FLAKY TEST !!!!!!!!!!!!", e)
			case m, ok := <-topic2:
				if ok {
					var val event
					require.NoError(t, m.Decode(&val))
					received2 <- val
				} else {
					close(received2)
				}
			}
		}

	}()

	go func() {
		<-start

		for {
			now := time.Now()
			evt := event{Time: now.UnixNano(), Message: fmt.Sprintf("Now is %v", now)}
			require.NoError(t, broker.Publish("local/instance", evt))
			evt = event{Time: now.Add(1 * time.Minute).UnixNano(), Message: fmt.Sprintf("Now is %v", now.Add(1*time.Minute))}
			require.NoError(t, broker.Publish("local/instancetest", evt))
		}
	}()

	close(start)

	// Test a few rounds to make sure all subscribers get the same messages each round.
	for i := 0; i < 5; i++ {
		b := <-received2
		a := <-received1
		require.NotNil(t, a)
		require.NotEqual(t, "", a.Message)
		require.NotEqual(t, a, b)
	}

	broker.Stop()

}

func TestBrokerSubscriberExactMatchTopic(t *testing.T) {
	type event struct {
		Time    int64
		Message string
	}

	socketFile := tempSocket()
	socket := "unix://broker" + socketFile

	broker, err := server.ListenAndServeOnSocket(socketFile)
	require.NoError(t, err)

	received1 := make(chan event)
	received2 := make(chan event)
	received3 := make(chan event)

	opts := Options{SocketDir: filepath.Dir(socketFile)}

	topic1, errs1, done1, err := Subscribe(socket, "local/instance", opts)
	require.NoError(t, err)
	go func() {
		for {
			select {
			case e := <-errs1:
				t.Log("!!!!!!!!!!!!!!!!! FLAKY TEST !!!!!!!!!!!!", e)
			case m, ok := <-topic1:
				if ok {
					var val event
					require.NoError(t, m.Decode(&val))
					received1 <- val
				} else {
					if received1 != nil {
						close(received1)
						received1 = nil
					}
				}
			}
		}
	}()

	topic2, errs2, done2, err := Subscribe(socket, "local/", opts)
	require.NoError(t, err)
	go func() {
		for {
			select {
			case e := <-errs2:
				t.Log("!!!!!!!!!!!!!!!!! FLAKY TEST !!!!!!!!!!!!", e)
			case m, ok := <-topic2:
				if ok {
					var val event
					require.NoError(t, m.Decode(&val))
					received2 <- val
				} else {
					if received2 != nil {
						close(received2)
						received2 = nil
					}
				}
			}
		}
	}()

	topic3, errs3, done3, err := Subscribe(socket, "local", opts)
	require.NoError(t, err)
	go func() {
		for {
			select {
			case e := <-errs3:
				t.Log("!!!!!!!!!!!!!!!!! FLAKY TEST !!!!!!!!!!!!", e)
			case m, ok := <-topic3:
				if ok {
					var val event
					require.NoError(t, m.Decode(&val))
					received3 <- val
				} else {
					if received3 != nil {
						close(received3)
						received3 = nil
					}
				}
			}
		}
	}()

	go func() {
		for {
			<-time.After(10 * time.Millisecond)
			now := time.Now()
			evt := event{Time: now.UnixNano(), Message: fmt.Sprintf("Now is %v", now)}
			require.NoError(t, broker.Publish("local/anotherinstance", evt))
			evt = event{Time: now.Add(1 * time.Minute).UnixNano(), Message: fmt.Sprintf("Now is %v", now.Add(1*time.Minute))}
			require.NoError(t, broker.Publish("local/instance", evt))
			evt = event{Time: now.Add(1 * time.Minute).UnixNano(), Message: fmt.Sprintf("Now is %v", now.Add(2*time.Minute))}
			require.NoError(t, broker.Publish("local", evt))
		}
	}()

	// Test a few rounds to make sure all subscribers get the same messages each round.
	for i := 0; i < 5; i++ {
		c := <-received3
		b := <-received2
		a := <-received1
		require.NotNil(t, a)
		require.NotEqual(t, "", a.Message)
		require.NotEqual(t, a, c)
		require.NotEqual(t, b, c)
		require.NotEqual(t, a, b)
	}

	close(done1)
	close(done2)
	close(done3)

	broker.Stop()
}

// This tests the case where the broker is mapped to url route (e.g. /events)
// In this case, we need to specify the path option in the connection options so that
// we can properly connect to the broker at the url prefix.
func TestBrokerMultiSubscriberCustomObjectConnectAtURLPrefix(t *testing.T) {

	type event struct {
		Time    int64
		Message string
	}

	socketFile := tempSocket()
	socket := "unix://broker" + socketFile

	broker, err := server.ListenAndServeOnSocket(socketFile, "/events")
	require.NoError(t, err)

	got404 := make(chan struct{})
	gotData := make(chan struct{})

	opts := Options{SocketDir: filepath.Dir(socketFile)}

	topic1, errs1, done1, err := Subscribe(socket, "local/", opts)
	require.NoError(t, err)
	go func() {
		for {
			select {
			case <-errs1:
				close(got404)
				return
			case <-topic1:
				panic("i shouldn't be here... i expect 404")
			}
		}
	}()

	opts.Path = "/events"
	topic2, errs2, done2, err := Subscribe(socket, "local/instance1", opts)
	require.NoError(t, err)
	go func() {
		for {
			select {

			case e := <-errs2:
				panic(e)
			case m := <-topic2:
				var val event
				require.NoError(t, m.Decode(&val))
				require.NotEqual(t, "", val.Message)
				close(gotData)
				return
			}
		}
	}()

	go func() {
		for {
			<-time.After(10 * time.Millisecond)

			now := time.Now()
			evt := event{Time: now.UnixNano(), Message: fmt.Sprintf("Now is %v", now)}
			require.NoError(t, broker.Publish("local/instance1", evt))
		}
	}()

	<-got404
	<-gotData

	close(done1)
	close(done2)
	broker.Stop()

}
