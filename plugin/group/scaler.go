package scaler

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/controller/util"
	"github.com/docker/libmachete/spi/instance"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Scaler is the spi of the scaler controller which mimics the behavior
// of an autoscaling group / scale set on AWS or Azure.
type Scaler interface {
	util.RunStop
	GetSize() uint32
	SetSize(size uint32)
	Describe() ([]instance.Description, error)
	Destroy() error
}

type scaler struct {
	pollInterval     time.Duration
	provisioner      instance.Plugin
	provisionRequest string
	tags             map[string]string
	size             uint32
	stop             chan bool
}

// NewFixedScaler creates a RunStop that monitors a group of instances on a provisioner, attempting to maintain a
// fixed size.
func NewFixedScaler(
	tags map[string]string,
	size uint32,
	pollInterval time.Duration,
	provisioner instance.Plugin,
	request string) (Scaler, error) {

	return &scaler{
		pollInterval:     pollInterval,
		provisioner:      provisioner,
		provisionRequest: request,
		tags:             tags,
		size:             size,
		stop:             make(chan bool),
	}, nil
}

type sortByID []instance.Description

func (n sortByID) Len() int {
	return len(n)
}

func (n sortByID) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

func (n sortByID) Less(i, j int) bool {
	return n[i].ID < n[j].ID
}

func (s *scaler) GetSize() uint32 {
	return s.size
}

func (s *scaler) SetSize(size uint32) {
	log.Infof("Set target size for group %v to %d", s.tags, size)
	atomic.StoreUint32(&s.size, size)
}

func (s *scaler) Describe() ([]instance.Description, error) {
	return s.provisioner.DescribeInstances(s.tags)
}

func (s *scaler) checkState() {
	log.Debugf("Checking instance size for group %v", s.tags)
	descriptions, err := s.provisioner.DescribeInstances(s.tags)
	if err != nil {
		log.Infof("Failed to check size of %v: %s", s.tags, err)
		return
	}

	log.Debugf("Found existing instances: %v", descriptions)

	actualSize := uint32(len(descriptions))

	switch {
	case actualSize == s.size:
		log.Debugf("Group %v has %d instances, no action is needed", s.tags, s.size)

	case actualSize > s.size:
		remove := actualSize - s.size
		log.Infof("Removing %d instances from group %v to reach desired %d", remove, s.tags, s.size)

		// Sorting first ensures that redundant operations are non-destructive.
		sort.Sort(sortByID(descriptions))

		toRemove := descriptions[:remove]

		grp := sync.WaitGroup{}
		for _, description := range toRemove {
			grp.Add(1)
			destroyID := description.ID
			go func() {
				defer grp.Done()

				err := s.provisioner.Destroy(destroyID)
				if err != nil {
					log.Errorf("Failed to destroy %s: %s", destroyID, err)
					return
				}
			}()
		}

		// Wait for all destroy calls to finish.
		// It is not imperative to avoid stepping on another removal operation by this routine
		// (within this process or another) since the selection of removal candidates is stable.
		// However, we do so here to mitigate redundant work and avoidable benign (but confusing) errors
		// when overlaps happen.
		grp.Wait()

	case actualSize < s.size:
		add := s.size - actualSize
		log.Infof("Adding %d instances to group %v to reach desired %d", add, s.tags, s.size)

		grp := sync.WaitGroup{}
		for i := 0; i < int(add); i++ {
			grp.Add(1)
			go func() {
				defer grp.Done()

				id, err := s.provisioner.Provision(s.provisionRequest, nil, s.tags)

				if err != nil {
					log.Errorf("Failed to grow group %v: %s", s.tags, err)
				} else {
					log.Infof("Created instance %s", *id)
				}
			}()
		}

		grp.Wait()
	}
}

func (s *scaler) Destroy() error {
	descriptions, err := s.provisioner.DescribeInstances(s.tags)
	if err != nil {
		return fmt.Errorf("Failed to check size of %v: %s", s.tags, err)
	}

	for _, inst := range descriptions {
		log.Infof("Destroying instance %s", inst.ID)
		err := s.provisioner.Destroy(inst.ID)
		if err != nil {
			return fmt.Errorf("Failed to destroy %s: %s", inst.ID, err)
		}
	}

	return nil
}

func (s *scaler) Run() {
	ticker := time.NewTicker(s.pollInterval)

	for {
		select {
		case <-ticker.C:
			s.checkState()
		case <-s.stop:
			ticker.Stop()
			return
		}
	}
}

func (s *scaler) Stop() {
	s.stop <- true
}
