package leader

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPoller(t *testing.T) {

	called := make(chan bool)
	getValue := make(chan bool)

	detector := NewPoller(10*time.Millisecond, func() (bool, error) {
		called <- true
		return <-getValue, nil
	})

	events, err := detector.Start()
	require.NoError(t, err)
	require.NotNil(t, events)

	for {
		didCall := <-called
		if didCall {
			getValue <- true
			break
		}
	}

	// Expect 1 event from the channel
	event := <-events
	require.Equal(t, Leader, event.Status)

	// Expect proper stopping
	detector.Stop()

	<-events // ensures properly closed channel.
}
