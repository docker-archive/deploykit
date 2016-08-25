package watcher

import (
	"bytes"
)

// WatchFunc is the function that watches for change in the resource.
// Use the channel provided to send the data and listen on the stop for termination.
type WatchFunc func(change chan<- []byte, stop <-chan struct{})

// EqualFunc determines equality of the resource between samples
type EqualFunc func(before, after []byte) bool

// ReactFunc is called when a change in the resource is detected.
type ReactFunc func(newData []byte)

// Watcher is the engine for detecting change in resource and performs some prescribed actions on change.
type Watcher struct {
	current []byte
	stop    chan struct{}
	watch   WatchFunc
	equal   EqualFunc
	react   ReactFunc
}

// New creates a watcher
func New(f WatchFunc, r ReactFunc) *Watcher {
	return new(Watcher).SetWatch(f).SetReact(r)
}

// SetWatch sets the watch function
func (w *Watcher) SetWatch(f WatchFunc) *Watcher {
	w.watch = f
	return w
}

// SetReact sets the function to call when reacting to change.
func (w *Watcher) SetReact(r ReactFunc) *Watcher {
	w.react = r
	return w
}

// SetEqual sets the function that determines version equality
func (w *Watcher) SetEqual(d EqualFunc) *Watcher {
	w.equal = d
	return w
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	if w.stop != nil {
		close(w.stop)
	}
}

// Run returns a channel to block on if desired.
func (w *Watcher) Run() <-chan struct{} {

	if w.stop == nil {
		w.stop = make(chan struct{})
	}
	if w.equal == nil {
		w.equal = bytes.Equal
	}

	inbound := make(chan []byte)
	stop := make(chan struct{})

	go func() {
		w.watch(inbound, stop)
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-w.stop:
				close(stop)
				return

			case newData := <-inbound:
				// Note that if the current value is nil, it's because it's the first
				// run. We should not react unless we actually see an observed change.
				// This will prevent unwanted side effects when the watcher is restarted.
				if w.current != nil && !w.equal(w.current, newData) {
					w.react(newData)
				}
				w.current = newData
			}
		}
	}()
	return done
}
