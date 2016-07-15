package scaler

import (
	"encoding/json"
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/spi/instance"
	"sort"
	"sync"
	"time"
)

// RunStop is an operation that may be Run (synchronously) and interrupted by calling Stop.
type RunStop interface {
	Run()

	Stop()
}

type scaler struct {
	pollInterval     time.Duration
	provisioner      instance.Provisioner
	provisionRequest string
	group            instance.GroupID
	count            uint
	stop             chan bool
}

type provisionRequest struct {
	Group instance.GroupID `json:"group"`
}

// NewFixedScaler creates a RunStop that monitors a group of instances on a provisioner, attempting to maintain a
// fixed count.
func NewFixedScaler(
	pollInterval time.Duration,
	provisioner instance.Provisioner,
	request string,
	count uint) (RunStop, error) {

	req := provisionRequest{}
	err := json.Unmarshal([]byte(request), &req)
	if err != nil {
		return nil, err
	}

	if req.Group == "" {
		return nil, errors.New("Group must not be empty")
	}

	return &scaler{
		pollInterval:     pollInterval,
		provisioner:      provisioner,
		provisionRequest: request,
		group:            req.Group,
		count:            count,
		stop:             make(chan bool),
	}, nil
}

type naturalSort []instance.ID

func (n naturalSort) Len() int {
	return len(n)
}

func (n naturalSort) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

func (n naturalSort) Less(i, j int) bool {
	return n[i] < n[j]
}

func (s *scaler) checkState() {
	log.Debugf("Checking instance count for group %s", s.group)
	instances, err := s.provisioner.ListGroup(s.group)
	if err != nil {
		log.Infof("Failed to check count of %s: %s", s.group, err)
		return
	}

	log.Debugf("Found existing instances: %v", instances)

	actualCount := uint(len(instances))

	switch {
	case actualCount == s.count:
		log.Infof("Group %s has %d instances, no action is needed", s.group, s.count)

	case actualCount > s.count:
		remove := actualCount - s.count
		log.Infof("Removing %d instances from group %s to reach desired %d", remove, s.group, s.count)

		// Sorting first ensures that redundant operations are non-destructive.
		sort.Sort(naturalSort(instances))

		toRemove := instances[:remove]

		group := sync.WaitGroup{}
		for _, instance := range toRemove {
			group.Add(1)
			destroyID := instance
			go func() {
				defer group.Done()

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
		group.Wait()

	case actualCount < s.count:
		add := s.count - actualCount
		log.Infof("Adding %d instances to group %s to reach desired %d", add, s.group, s.count)

		group := sync.WaitGroup{}
		for i := 0; i < int(add); i++ {
			group.Add(1)
			go func() {
				defer group.Done()

				id, err := s.provisioner.Provision(s.provisionRequest)

				if err != nil {
					log.Errorf("Failed to grow group %s: %s", s.group, err)
				} else {
					log.Infof("Created instance %s", *id)
				}
			}()
		}

		group.Wait()
	}
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
