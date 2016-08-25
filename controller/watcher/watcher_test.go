package watcher

import (
	"bytes"
	"testing"
	"time"
)

func TestWatchRunAndStopProperly(t *testing.T) {
	w := new(Watcher).SetWatch(
		func(change chan<- []byte, stop <-chan struct{}) {
			tick := time.Tick(1 * time.Millisecond)
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				case <-tick:
					if i == 5 {
						change <- []byte("stop")
					} else {
						change <- []byte("same data")
					}
				}
			}
		})
	w.SetReact(
		func(newData []byte) {
			t.Log("react")
			if bytes.Equal([]byte("stop"), newData) {
				w.Stop()
			}
		})

	<-w.Run() // hangs indefinitely until the Stop is called in another goroutine.
}
