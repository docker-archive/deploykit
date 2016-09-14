package group

import (
	"errors"
	log "github.com/Sirupsen/logrus"
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

func desiredAndUndesiredInstances(
	instances []instance.Description, settings groupSettings) ([]instance.Description, []instance.Description) {

	desiredHash := settings.config.InstanceHash()
	desired := []instance.Description{}
	undesired := []instance.Description{}
	for _, inst := range instances {
		actualConfig, specified := inst.Tags[configTag]
		if specified && actualConfig == desiredHash {
			desired = append(desired, inst)
		} else {
			undesired = append(undesired, inst)
		}
	}

	return desired, undesired
}

type rollingupdate struct {
	desc       string
	scaled     Scaled
	updatingTo groupSettings
	stop       chan bool
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
			instances, err := r.scaled.List()
			if err != nil {
				return err
			}

			matching, _ := desiredAndUndesiredInstances(instances, r.updatingTo)

			// We are only concerned with the expected number of instances in the desired state.
			// For example, if the original state of the group was failing to successfully create instances,
			// old instances may never show up.  We should, however, avoid proceeding if the new instances
			// are not showing up.
			if len(matching) >= int(expectedNewInstances) {
				return nil
			}

			log.Info("Waiting for scaler to quiesce")

			// TODO(wfarner): Poll ProvisionHelper.Healthy here.

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

	instances, err := r.scaled.List()
	if err != nil {
		return err
	}

	desired, _ := desiredAndUndesiredInstances(instances, r.updatingTo)
	expectedNewInstances := len(desired)

	for {
		err := r.waitUntilQuiesced(pollInterval, minInt(expectedNewInstances, int(r.updatingTo.config.Size)))
		if err != nil {
			return err
		}
		log.Info("Scaler has quiesced")

		instances, err := r.scaled.List()
		if err != nil {
			return err
		}

		_, undesiredInstances := desiredAndUndesiredInstances(instances, r.updatingTo)

		if len(undesiredInstances) == 0 {
			break
		}

		log.Infof("Found %d undesired instances", len(undesiredInstances))

		// Sort instances first to ensure predictable destroy order.
		sort.Sort(sortByID(undesiredInstances))

		// TODO(wfarner): Provide a mechanism to gracefully drain instances.
		// TODO(wfarner): Make the 'batch size' configurable, but not for manager role groups.
		r.scaled.Destroy(undesiredInstances[0].ID)

		expectedNewInstances++
	}

	return nil
}

func (r *rollingupdate) Stop() {
	r.stop <- true
}
