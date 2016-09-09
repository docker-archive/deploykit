package scaler

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/controller/util"
	"github.com/docker/libmachete/spi/instance"
	"sort"
	"sync"
	"time"
)

// Scaled is a collection of instances that can be scaled up and down.
type Scaled interface {
	CreateOne()

	Destroy(id instance.ID) error

	List() ([]instance.ID, error)
}

// Scaler is the spi of the scaler controller which mimics the behavior
// of an autoscaling group / scale set on AWS or Azure.
type Scaler interface {
	util.RunStop
	SetSize(size uint32)
}

type scaler struct {
	scaled       Scaled
	size         uint32
	pollInterval time.Duration
	lock         sync.Mutex
	stop         chan bool
}

// NewAdjustableScaler creates a RunStop that monitors a group of instances on a provisioner, attempting to maintain a
// desired size.
func NewAdjustableScaler(scaled Scaled, size uint32, pollInterval time.Duration) Scaler {
	return &scaler{
		scaled:       scaled,
		size:         size,
		pollInterval: pollInterval,
		stop:         make(chan bool),
	}
}

func (s *scaler) SetSize(size uint32) {
	s.lock.Lock()
	defer s.lock.Unlock()

	log.Infof("Set target size to %d", size)
	s.size = size
}

func (s *scaler) getSize() uint32 {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.size
}

func (s *scaler) Stop() {
	s.stop <- true
}

func (s *scaler) Run() {
	ticker := time.NewTicker(s.pollInterval)

	for {
		select {
		case <-ticker.C:
			ids, err := s.scaled.List()
			if err != nil {
				log.Infof("Failed to check size of group: %s", err)
				return
			}

			log.Debugf("Found existing instances: %v", ids)

			s.convergeOnce(ids)

		case <-s.stop:
			ticker.Stop()
			return
		}
	}
}

func (s *scaler) convergeOnce(ids []instance.ID) {
	grp := sync.WaitGroup{}

	actualSize := uint32(len(ids))
	desiredSize := s.getSize()
	switch {
	case actualSize == desiredSize:
		log.Debugf("Group has %d instances, no action is needed", desiredSize)

	case actualSize > desiredSize:
		remove := actualSize - desiredSize
		log.Infof("Removing %d instances from group to reach desired %d", remove, desiredSize)

		// Sorting first ensures that redundant operations are non-destructive.
		sort.Sort(sortByID(ids))

		for _, id := range ids[:remove] {
			grp.Add(1)
			destroy := id
			go func() {
				defer grp.Done()
				s.scaled.Destroy(destroy)
			}()
		}

	case actualSize < desiredSize:
		add := desiredSize - actualSize
		log.Infof("Adding %d instances to group to reach desired %d", add, desiredSize)

		for i := 0; i < int(add); i++ {
			grp.Add(1)
			go func() {
				defer grp.Done()

				s.scaled.CreateOne()
			}()
		}
	}

	// Wait for outstanding actions to finish.
	// It is not imperative to avoid stepping on another removal operation by this routine
	// (within this process or another) since the selection of removal candidates is stable.
	// However, we do so here to mitigate redundant work and avoidable benign (but confusing) errors
	// when overlaps happen.
	grp.Wait()
}
