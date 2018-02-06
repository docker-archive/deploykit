package group

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// TODO(wfarner): Converge this implementation with scaler.go, they share a lot of behavior.

type quorum struct {
	id           group.ID
	scaled       Scaled
	LogicalIDs   []instance.LogicalID
	pollInterval time.Duration
	stop         chan bool
}

// NewQuorum creates a supervisor for a group of instances operating in a quorum.
func NewQuorum(id group.ID, scaled Scaled, logicalIDs []instance.LogicalID, pollInterval time.Duration) Supervisor {
	return &quorum{
		id:           id,
		scaled:       scaled,
		LogicalIDs:   logicalIDs,
		pollInterval: pollInterval,
		stop:         make(chan bool),
	}
}

func (q *quorum) PlanUpdate(scaled Scaled, settings groupSettings, newSettings groupSettings) (updatePlan, error) {
	if !reflect.DeepEqual(settings.config.Allocation.LogicalIDs, newSettings.config.Allocation.LogicalIDs) {
		return nil, errors.New("Logical ID changes to a quorum is not currently supported")
	}

	// Determine how many instances are not at the desired instance configuration
	instances, err := labelAndList(q.scaled)
	if err != nil {
		return nil, err
	}
	_, undesired := desiredAndUndesiredInstances(instances, newSettings)
	if len(undesired) == 0 {
		// This is a no-op update because the instance configuration is unchanged
		return &noopUpdate{}, nil
	}

	return &rollingupdate{
		desc: fmt.Sprintf(
			"Performing a rolling update on %d instances",
			len(undesired)),
		scaled:       scaled,
		updatingFrom: settings,
		updatingTo:   newSettings,
		stop:         make(chan bool),
	}, nil
}

func (q *quorum) Stop() {
	close(q.stop)
}

func (q *quorum) Run() {
	ticker := time.NewTicker(q.pollInterval)

	q.converge()
	for {
		select {
		case <-ticker.C:
			q.converge()

		case <-q.stop:
			ticker.Stop()
			return
		}
	}
}

func (q *quorum) ID() group.ID {
	return q.id
}

func (q *quorum) Size() uint {
	return uint(len(q.LogicalIDs))
}

func (q *quorum) converge() {
	descriptions, err := labelAndList(q.scaled)
	if err != nil {
		log.Error("Failed to check to group", "err", err)
		return
	}

	log.Debug("Found existing instances", "groupID", q.ID(), "descriptions", descriptions, "V", debugV)

	unknownIPs := []instance.Description{}
	for _, description := range descriptions {
		if description.LogicalID == nil {
			log.Warn("No logical ID", "description", description, "id", description.ID)
			continue
		}

		matched := false
		for _, expectedID := range q.LogicalIDs {
			if expectedID == *description.LogicalID {
				matched = true
			}
		}
		if !matched {
			unknownIPs = append(unknownIPs, description)
		}
	}

	grp := sync.WaitGroup{}

	for _, ip := range unknownIPs {
		unknownInstance := ip
		log.Warn("Destroying instances with unknown IP address", "instance", unknownInstance)

		grp.Add(1)
		go func() {
			defer grp.Done()
			q.scaled.Destroy(unknownInstance, instance.Termination)
		}()
	}

	missingIDs := []instance.LogicalID{}
	for _, expectedID := range q.LogicalIDs {
		matched := false
		for _, description := range descriptions {
			if description.LogicalID == nil {
				continue
			}

			if expectedID == *description.LogicalID {
				matched = true
			}
		}
		if !matched {
			missingIDs = append(missingIDs, expectedID)
		}
	}

	for _, missingID := range missingIDs {
		log.Info("Logical ID is missing, provisioning new instance", "instance", missingID)
		id := missingID

		grp.Add(1)
		go func() {
			defer grp.Done()

			q.scaled.CreateOne(&id)
		}()
	}

	grp.Wait()
}
