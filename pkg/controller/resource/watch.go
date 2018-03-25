package resource

import (
	"context"
	"sync"
)

type watchers []chan<- struct{}

// Notify notifies the watchers
func (w watchers) Notify() {
	for _, ch := range w {
		close(ch)
	}
}

// Watch manages a collection of watchers by name
type Watch struct {
	watchers map[string]watchers

	lock sync.RWMutex
}

// Add adds a watcher by name
func (w *Watch) Add(name string, watcher chan<- struct{}) {
	w.lock.Lock()
	defer w.lock.Unlock()

	if w.watchers == nil {
		w.watchers = map[string]watchers{}
	}
	w.watchers[name] = append(w.watchers[name], watcher)
}

// Notify notifies all the watchers.  This is one time only.  The entry will be cleared afterwards.
func (w *Watch) Notify(name string) {
	w.notify(name)
	w.clear(name)
}

func (w *Watch) notify(name string) {
	w.lock.RLock()
	defer w.lock.RUnlock()

	watchers, has := w.watchers[name]

	if !has || len(watchers) == 0 {
		return
	}
	log.Debug("Notify", "name", name, "watchers", watchers, "V", debugV)
	watchers.Notify()
}

func (w *Watch) clear(name string) {
	w.lock.Lock()
	defer w.lock.Unlock()
	delete(w.watchers, name)
}

// Watchers is a slice of receiver channels
type Watchers []<-chan struct{}

// FanIn merges all the watchers into a single signal.  The channel is closed only when every watcher is signaled.
func (watchers Watchers) FanIn(ctx context.Context) <-chan struct{} {
	var wg sync.WaitGroup
	out := make(chan struct{})

	wg.Add(len(watchers))
	for _, ch := range watchers {
		go func(c <-chan struct{}) {
			defer wg.Done()
			select {
			case <-c:
			case <-ctx.Done():
			}
		}(ch)
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}
