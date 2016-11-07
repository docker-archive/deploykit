package leader

import (
	"time"

	log "github.com/Sirupsen/logrus"
)

type poller struct {
	leaderChan   chan Leadership
	pollInterval time.Duration
	tick         <-chan time.Time
	stop         chan struct{}
	pollFunc     CheckLeaderFunc
}

// NewPoller returns a detector implementation given the poll interval and function that polls
func NewPoller(pollInterval time.Duration, f CheckLeaderFunc) Detector {
	return &poller{
		pollInterval: pollInterval,
		tick:         time.Tick(pollInterval),
		pollFunc:     f,
	}
}

// Start implements Detect.Start
func (l *poller) Start() (<-chan Leadership, error) {
	if l.leaderChan != nil {
		return l.leaderChan, nil
	}

	l.leaderChan = make(chan Leadership)
	l.stop = make(chan struct{})

	go l.poll()
	return l.leaderChan, nil
}

// Stop implements Detect.Stop
func (l *poller) Stop() {
	if l.stop != nil {
		close(l.stop)
	}
}

func (l *poller) poll() {
	for {
		select {

		case <-l.tick:

			isLeader, err := l.pollFunc()
			event := Leadership{}
			if err != nil {
				event.Status = StatusUnknown
				event.Error = err
			} else {
				if isLeader {
					event.Status = StatusLeader
				} else {
					event.Status = StatusNotLeader
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
