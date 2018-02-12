package group

import (
	"fmt"
	"sort"
	"sync"
	"time"

	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
)

type scaler struct {
	options        group_types.Options
	id             group.ID
	scaled         Scaled
	size           uint
	pollInterval   time.Duration
	maxParallelNum uint
	lock           sync.Mutex
	stop           chan bool
}

// NewScalingGroup creates a supervisor that monitors a group of instances on a provisioner, attempting to maintain a
// desired size.
func NewScalingGroup(id group.ID, scaled Scaled, size uint, pollInterval time.Duration, maxParallelNum uint) Supervisor {
	return &scaler{
		id:             id,
		scaled:         scaled,
		size:           size,
		pollInterval:   pollInterval,
		maxParallelNum: maxParallelNum,
		stop:           make(chan bool),
	}
}

func (s *scaler) PlanUpdate(scaled Scaled, settings groupSettings, newSettings groupSettings) (updatePlan, error) {
	sizeChange := int(newSettings.config.Allocation.Size) - int(settings.config.Allocation.Size)

	instances, err := labelAndList(s.scaled)
	if err != nil {
		return nil, err
	}

	desired, undesired := desiredAndUndesiredInstances(instances, newSettings)

	plan := scalerUpdatePlan{
		originalSize: settings.config.Allocation.Size,
		newSize:      newSettings.config.Allocation.Size,
		scaler:       s,
		rollingPlan:  noopUpdate{},
	}

	switch {
	case sizeChange == 0:
		rollCount := len(undesired)
		if rollCount == 0 {
			// This is a no-op update because:
			//  - the instance configuration is unchanged
			//  - the group contains no instances with an undesired state
			//  - the group size is unchanged
			return &noopUpdate{}, nil
		}
		// Perform a rolling update on the instances that do not match the desired
		plan.desc = fmt.Sprintf("Performing a rolling update on %d instances", rollCount)

	case sizeChange < 0:
		rollCount := int(newSettings.config.Allocation.Size) - len(desired)
		if rollCount < 0 {
			rollCount = 0
		}

		if rollCount == 0 {
			plan.desc = fmt.Sprintf(
				"Terminating %d instances to reduce the group size to %d",
				int(sizeChange)*-1,
				newSettings.config.Allocation.Size)
		} else {
			plan.desc = fmt.Sprintf(
				"Terminating %d instances to reduce the group size to %d,"+
					" then performing a rolling update on %d instances",
				int(sizeChange)*-1,
				newSettings.config.Allocation.Size,
				rollCount)
		}

	case sizeChange > 0:
		rollCount := len(undesired)

		if rollCount == 0 {
			plan.desc = fmt.Sprintf(
				"Adding %d instances to increase the group size to %d",
				sizeChange,
				newSettings.config.Allocation.Size)
		} else {
			plan.desc = fmt.Sprintf(
				"Performing a rolling update on %d instances,"+
					" then adding %d instances to increase the group size to %d",
				rollCount,
				sizeChange,
				newSettings.config.Allocation.Size)
		}
	}

	plan.rollingPlan = &rollingupdate{
		scaled:       scaled,
		updatingFrom: settings,
		updatingTo:   newSettings,
		stop:         make(chan bool),
	}

	return plan, nil
}

type scalerUpdatePlan struct {
	desc         string
	originalSize uint
	newSize      uint
	rollingPlan  updatePlan
	scaler       *scaler
}

func (s scalerUpdatePlan) Explain() string {
	return s.desc
}

func (s scalerUpdatePlan) Run(pollInterval time.Duration, updating group_types.Updating) error {

	// If the number of instances is being decreased, first lower the group size.  This eliminates
	// instances that would otherwise be rolled first, avoiding unnecessary work.
	// We could further optimize by selecting undesired instances to destroy, for example if the
	// scaler already has a mix of desired and undesired instances.
	if s.newSize < s.originalSize {
		s.scaler.SetSize(s.newSize)
	}

	if err := s.rollingPlan.Run(pollInterval, updating); err != nil {
		return err
	}

	// Rolling has completed.  If the update included a group size increase, perform that now.
	if s.newSize > s.originalSize {
		s.scaler.SetSize(s.newSize)
	}

	return nil
}

func (s scalerUpdatePlan) Stop() {
	s.rollingPlan.Stop()
}

func (s *scaler) SetSize(size uint) {
	s.lock.Lock()
	defer s.lock.Unlock()

	log.Info("Set target size", "size", size)
	s.size = size
}

func (s *scaler) getSize() uint {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.size
}

func (s *scaler) SetMaxParallelNum(psize uint) {
	s.lock.Lock()
	defer s.lock.Unlock()

	log.Info("Set max parallel instance creation", "max", psize)
	s.maxParallelNum = psize
}

func (s *scaler) getMaxParallelNum() uint {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.maxParallelNum
}

func (s *scaler) Stop() {
	close(s.stop)
}

func (s *scaler) Run() {
	ticker := time.NewTicker(s.pollInterval)

	s.converge()
	for {
		select {
		case <-ticker.C:
			s.converge()
		case <-s.stop:
			ticker.Stop()
			return
		}
	}
}

func (s *scaler) ID() group.ID {
	return s.id
}

func (s *scaler) Size() uint {
	return s.getSize()
}

func (s *scaler) waitIfReachParallelLimit(current int, batch *sync.WaitGroup) {
	if s.maxParallelNum > 0 && (current+1)%int(s.maxParallelNum) == 0 {
		log.Warn("Reach limit parallel instance operation number, waiting", "max", s.maxParallelNum)
		batch.Wait()
	}
	return
}

func (s *scaler) converge() {
	descriptions, err := labelAndList(s.scaled)
	if err != nil {
		log.Error("Failed to list group instances", "err", err)
		return
	}

	log.Debug("Found existing instances", "descriptions", descriptions, "V", debugV)

	grp := sync.WaitGroup{}

	actualSize := uint(len(descriptions))
	desiredSize := s.getSize()
	switch {
	case actualSize == desiredSize:
		log.Debug("No action - Group has enough instances", "desired", desiredSize)

	case actualSize > desiredSize:
		remove := actualSize - desiredSize
		log.Info("Removing instances", "actualSize", actualSize, "remove", remove, "desired", desiredSize)

		sorted := make([]instance.Description, len(descriptions))
		copy(sorted, descriptions)

		// Sorting first ensures that redundant operations are non-destructive.
		sort.Sort(sortByID{list: sorted})

		// TODO(wfarner): Consider favoring removal of instances that do not match the desired configuration by
		// injecting a sorter.
		for i, toDestroy := range sorted[:remove] {
			grp.Add(1)
			destroy := toDestroy
			go func() {
				defer grp.Done()
				s.scaled.Destroy(destroy, instance.Termination)
			}()
			s.waitIfReachParallelLimit(i, &grp)
		}

	case actualSize < desiredSize:
		add := desiredSize - actualSize
		log.Info("Adding instances to group", "actualSize", actualSize, "add", add, "desired", desiredSize)

		for i := 0; i < int(add); i++ {
			grp.Add(1)
			go func() {
				defer grp.Done()

				s.scaled.CreateOne(nil)
			}()
			s.waitIfReachParallelLimit(i, &grp)
		}
	}

	// Wait for outstanding actions to finish.
	// It is not imperative to avoid stepping on another removal operation by this routine
	// (within this process or another) since the selection of removal candidates is stable.
	// However, we do so here to mitigate redundant work and avoidable benign (but confusing) errors
	// when overlaps happen.
	grp.Wait()
}
