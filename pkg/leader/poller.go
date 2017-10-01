package leader

import (
	"sync"
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
)

var log = logutil.New("module", "leader/poll")

// Poller is the entity that polls for different backend / control planes to determine leadership
type Poller struct {
	pollInterval time.Duration
	tick         <-chan time.Time
	stop         chan struct{}
	pollFunc     CheckLeaderFunc
	store        Store
	lock         sync.Mutex
	receivers    []chan Leadership
}

// NewPoller returns a detector implementation given the poll interval and function that polls
func NewPoller(pollInterval time.Duration, f CheckLeaderFunc) *Poller {
	return &Poller{
		pollInterval: pollInterval,
		tick:         time.Tick(pollInterval),
		pollFunc:     f,
		receivers:    []chan Leadership{},
	}
}

// Receive returns a channel to receive on. It broadcasts so that all channel receivers will get the same message.
func (l *Poller) Receive() <-chan Leadership {
	l.lock.Lock()
	defer l.lock.Unlock()
	c := make(chan Leadership)
	l.receivers = append(l.receivers, c)
	return c
}

func (l *Poller) startPoll() {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.stop = make(chan struct{})
	go l.poll()
}

// Start implements Detect.Start
func (l *Poller) Start() (<-chan Leadership, error) {
	if len(l.receivers) > 0 {
		// means it's started polling
		return l.Receive(), nil
	}
	c := l.Receive()
	l.startPoll()
	return c, nil
}

// Stop implements Detect.Stop
func (l *Poller) Stop() {
	if l.stop != nil {
		close(l.stop)
	}
}

func (l *Poller) poll() {
	for {

		isLeader, err := l.pollFunc()
		event := Leadership{}
		if err != nil {
			event.Status = Unknown
			event.Error = err
		} else {
			if isLeader {
				event.Status = Leader

			} else {
				event.Status = NotLeader
			}
		}

		for _, receiver := range l.receivers {
			receiver <- event
		}

		select {

		case <-l.tick:
		case <-l.stop:

			log.Info("Stopping leadership check")
			clean := l.receivers
			l.receivers = nil
			for _, receiver := range clean {
				close(receiver)
			}
			l.stop = nil
			return
		}
	}
}
