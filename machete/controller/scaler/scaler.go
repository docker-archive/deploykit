package scaler

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/provisioners/spi"
	"sort"
	"sync"
	"time"
)

// Scaler is an auto-scaler that monitors individual instance groups.
type Scaler interface {
	// MaintainCount will poll the `provisioner` periodically to ensure that an instance group has exactly `count`
	// instances.  When the count is found to be different, the provisioner will be instructed to add or remove
	// instances to return to the desired instance count.
	MaintainCount(provisioner spi.Provisioner, id spi.GroupID, count uint) error
}

type scaler struct {
	pollInterval time.Duration
	ticker       *time.Ticker
	lock         sync.Mutex
}

type naturalSort []spi.InstanceID

func (n naturalSort) Len() int {
	return len(n)
}

func (n naturalSort) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

func (n naturalSort) Less(i, j int) bool {
	return n[i] < n[j]
}

func stableSelect(instances []spi.InstanceID, count uint) []spi.InstanceID {
	if count > uint(len(instances)) {
		panic("scaler: too many values to select")
	}

	// Sorting first ensures that redundant operations are non-destructive.
	sort.Sort(naturalSort(instances))

	return instances[:count]
}

func destroy(provisioner spi.Provisioner, instance spi.InstanceID) {
	events, err := provisioner.DestroyInstance(string(instance))
	if err != nil {
		log.Errorf("Failed to destroy %s: %s", instance, err)
		return
	}

	for event := range events {
		log.Debug(event)
	}
}

func (s *scaler) maintainCountSync(provisioner spi.Provisioner, group spi.GroupID, count uint) {
	for range s.ticker.C {
		log.Debugf("Checking instance count for group %s", group)
		instances, err := provisioner.GetInstances(group)
		if err != nil {
			log.Infof("Failed to check count of %s: %s", group, err)
			continue
		}

		actualCount := uint(len(instances))

		switch {
		case actualCount == count:
			log.Debugf("Group %s has %d instances, no action is needed", group, count)

		case actualCount > count:
			remove := actualCount - count
			log.Infof("Removing %d instances from group %s to reach desired %d", remove, group, count)

			group := sync.WaitGroup{}
			for _, instance := range stableSelect(instances, remove) {
				group.Add(1)
				destroyID := instance
				go func() {
					defer group.Done()
					destroy(provisioner, destroyID)
				}()
			}

			// Wait for all destroy calls to finish.
			// It is not imperative to avoid stepping on another removal operation by this routine
			// (within this process or another) since the selection of removal candidates is stable.
			// However, we do so here to mitigate redundant work and avoidable benign (but confusing) errors
			// when overlaps happen.
			group.Wait()

		case actualCount < count:
			add := count - actualCount
			log.Infof("Adding %d instances to group %s to reach desired %d", add, group, count)
			err = provisioner.AddGroupInstances(group, add)
			if err != nil {
				log.Errorf("Failed to grow group %s: %s", group, err)
			}
		}
	}
}

func (s *scaler) MaintainCount(provisioner spi.Provisioner, id spi.GroupID, count uint) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.ticker != nil {
		return fmt.Errorf("Already monitoring %s", id)
	}

	s.ticker = time.NewTicker(s.pollInterval)
	go s.maintainCountSync(provisioner, id, count)

	return nil
}

func (s *scaler) Stop() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.ticker == nil {
		return fmt.Errorf("Scaler is not active")
	}

	s.ticker.Stop()
	s.ticker = nil

	return nil
}
