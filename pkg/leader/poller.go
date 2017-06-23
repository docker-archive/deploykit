package leader

import (
	"net/url"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

type Poller struct {
	leaderChan   chan Leadership
	pollInterval time.Duration
	tick         <-chan time.Time
	stop         chan struct{}
	pollFunc     CheckLeaderFunc
	location     *url.URL
	store        Store
	lock         sync.Mutex
}

// NewPoller returns a detector implementation given the poll interval and function that polls
func NewPoller(pollInterval time.Duration, f CheckLeaderFunc) *Poller {
	return &Poller{
		pollInterval: pollInterval,
		tick:         time.Tick(pollInterval),
		pollFunc:     f,
	}
}

// ReportLocation tells the poller to report its location when it becomes the leader.
func (l *Poller) ReportLocation(url *url.URL, store Store) {
	if url == nil {
		panic("leader poller url cannot be nil")
	}
	if store == nil {
		panic("leader poller store cannot be nil")
	}

	l.lock.Lock()
	defer l.lock.Unlock()

	l.location = url
	l.store = store
}

// Start implements Detect.Start
func (l *Poller) Start() (<-chan Leadership, error) {
	if l.leaderChan != nil {
		return l.leaderChan, nil
	}

	l.leaderChan = make(chan Leadership)
	l.stop = make(chan struct{})

	go l.poll()
	return l.leaderChan, nil
}

// Stop implements Detect.Stop
func (l *Poller) Stop() {
	if l.stop != nil {
		close(l.stop)
	}
}

func (l *Poller) poll() {
	for {
		select {

		case <-l.tick:

			isLeader, err := l.pollFunc()
			event := Leadership{}
			if err != nil {
				event.Status = Unknown
				event.Error = err
			} else {
				if isLeader {
					event.Status = Leader

					if l.location != nil {
						l.store.UpdateLocation(l.location)
					}

				} else {
					event.Status = NotLeader
				}
			}

			l.leaderChan <- event

		case <-l.stop:
			log.Infoln("Stopping leadership check")
			close(l.leaderChan)
			l.leaderChan = nil
			l.stop = nil
			return
		}
	}
}
