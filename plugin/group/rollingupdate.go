package scaler

import (
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
	"sort"
	"time"
)

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func planRollingUpdate(id group.ID, context *groupContext, newProps groupProperties) (updatePlan, error) {

	instances, err := context.scaled.describe()
	if err != nil {
		return nil, err
	}

	sizeChange := int(newProps.Size) - int(context.properties.Size)

	_, undesiredInstances := desiredAndUndesiredInstances(instances, newProps)
	rollCount := minInt(len(undesiredInstances), int(newProps.Size))

	update := rollingupdate{
		context:  context,
		newProps: newProps,
		stop:     make(chan bool),
	}

	if rollCount == 0 && sizeChange == 0 {
		if instanceConfigHash(context.properties.InstancePluginProperties) ==
			instanceConfigHash(newProps.InstancePluginProperties) {

			// This is a no-op update because:
			//  - the instance configuration is unchanged
			//  - the group contains no instances with an undesired state
			//  - the group size is unchanged
			return &noexecUpdate{desc: "Noop"}, nil
		}

		// This case likely occurs because a group was created in a way that no instances are being created.
		// We proceed with the update here, which will likely only change the target configuration in the
		// scaler.
		update.desc = "Adjusts the instance configuration, no restarts necessary"
		return &update, nil
	}

	rollDesc := fmt.Sprintf("a rolling update on %d instances", rollCount)

	switch {
	case sizeChange == 0:
		update.desc = fmt.Sprintf("Performs %s", rollDesc)
	case sizeChange < 0:
		if rollCount > 0 {
			update.desc = fmt.Sprintf(
				"Terminates %d instances to reduce the group size to %d, then performs %s",
				int(sizeChange)*-1,
				newProps.Size,
				rollDesc)
		} else {
			update.desc = fmt.Sprintf(
				"Terminates %d instances to reduce the group size to %d",
				int(sizeChange)*-1,
				newProps.Size)
		}

	case sizeChange > 0:
		if rollCount > 0 {
			update.desc = fmt.Sprintf(
				"Performs %s, then adds %d instances to increase the group size to %d",
				rollDesc,
				sizeChange,
				newProps.Size)
		} else {
			update.desc = fmt.Sprintf(
				"Adds %d instances to increase the group size to %d",
				sizeChange,
				newProps.Size)
		}
	}

	return &update, nil
}

func desiredAndUndesiredInstances(instances []instance.Description, props groupProperties) ([]instance.ID, []instance.ID) {
	desiredConfig := instanceConfigHash(props.InstancePluginProperties)

	desired := []instance.ID{}
	undesired := []instance.ID{}
	for _, inst := range instances {
		actualConfig, specified := inst.Tags[configTag]
		if specified && actualConfig == desiredConfig {
			desired = append(desired, inst.ID)
		} else {
			undesired = append(undesired, inst.ID)
		}
	}

	return desired, undesired
}

type rollingupdate struct {
	desc     string
	context  *groupContext
	newProps groupProperties
	stop     chan bool
}

func (r rollingupdate) Explain() string {
	return r.desc
}

func (r *rollingupdate) waitUntilQuiesced(pollInterval time.Duration, expectedNewInstances int) error {
	// Block until the expected number of instances in the desired state are ready.  Updates are unconcerned with
	// the health of instances in the undesired state.  This allows a user to dig out of a hole where the original
	// state of the group is bad, and instances are not reporting as healthy.

	ticker := time.NewTicker(pollInterval)
	for {
		select {
		case <-ticker.C:
			// Gather instances in the scaler with the desired state
			// Check:
			//   - that the scaler has the expected number of instances
			//   - instances with the desired config are healthy (e.g. represented in `swarm node ls`)

			// TODO(wfarner): Get this information from the scaler to reduce redundant network calls.
			instances, err := r.context.scaled.describe()
			if err != nil {
				return err
			}

			matching, _ := desiredAndUndesiredInstances(instances, r.newProps)

			// We are only concerned with the expected number of instances in the desired state.
			// For example, if the original state of the group was failing to successfully create instances,
			// old instances may never show up.  We should, however, avoid proceeding if the new instances
			// are not showing up.
			if len(matching) >= int(expectedNewInstances) {
				return nil
			}

			log.Info("Waiting for scaler to quiesce")

			// TODO(wfarner): Provide a mechanism for health feedback.

		case <-r.stop:
			ticker.Stop()
			return errors.New("Update halted by user")
		}
	}
}

// Run identifies instances not matching the desired state and destroys them one at a time until all instances in the
// group match the desired state, with the desired number of instances.
// TODO(wfarner): Make this routine more resilient to transient errors.
func (r *rollingupdate) Run(pollInterval time.Duration) error {
	originalSize := r.context.properties.Size

	// If the number of instances is being decreased, first lower the group size.  This eliminates
	// instances that would otherwise be rolled first, avoiding unnecessary work.
	// We could further optimize by selecting undesired instances to destroy, for example if the
	// scaler already has a mix of desired and undesired instances.
	if originalSize > r.newProps.Size {
		r.context.scaler.SetSize(r.newProps.Size)
	}

	instances, err := r.context.scaled.describe()
	if err != nil {
		return err
	}

	desired, _ := desiredAndUndesiredInstances(instances, r.newProps)
	expectedNewInstances := len(desired)

	r.context.setProperties(&r.newProps)
	r.context.scaled.setProvisionTemplate(r.newProps.InstancePluginProperties, identityTags(r.newProps))

	for {
		err := r.waitUntilQuiesced(pollInterval, minInt(expectedNewInstances, int(r.newProps.Size)))
		if err != nil {
			return err
		}
		log.Info("Scaler has quiesced")

		instances, err := r.context.scaled.describe()
		if err != nil {
			return err
		}

		_, undesiredInstances := desiredAndUndesiredInstances(instances, r.newProps)

		if len(undesiredInstances) == 0 {
			break
		}

		log.Infof("Found %d undesired instances", len(undesiredInstances))

		// Sort instances first to ensure predictable destroy order.
		sort.Sort(sortByID(undesiredInstances))

		// TODO(wfarner): Provide a mechanism to gracefully drain instances.
		// TODO(wfarner): Make the 'batch size' configurable.
		err = r.context.scaled.Destroy(undesiredInstances[0])
		if err != nil {
			return err
		}
		expectedNewInstances++
	}

	// Rolling has completed.  If the update included a group size increase, perform that now.
	if originalSize < r.newProps.Size {
		r.context.scaler.SetSize(r.newProps.Size)
	}

	return nil
}

func (r *rollingupdate) Stop() {
	r.stop <- true
}
