package group

import (
	"errors"
	"fmt"
	"sort"
	"time"

	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
)

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func selfUpdatePolicy(settings groupSettings) group_types.PolicyLeaderSelfUpdate {
	if settings.options.PolicyLeaderSelfUpdate == nil {
		return group_types.PolicyLeaderSelfUpdateLast // default policy
	}
	return *settings.options.PolicyLeaderSelfUpdate
}

func isSelf(inst instance.Description, settings groupSettings) bool {
	if settings.self != nil {
		if inst.LogicalID != nil && *inst.LogicalID == *settings.self {
			return true
		}
		if v, has := inst.Tags[instance.LogicalIDTag]; has {
			return string(*settings.self) == v
		}
	}
	return false
}

func canDestroy(inst instance.Description, settings groupSettings) bool {
	if !isSelf(inst, settings) {
		// not running on the same node, so ok to destroy
		return true
	}
	// self update - never - means we'd never update the self node.
	return selfUpdatePolicy(settings) != group_types.PolicyLeaderSelfUpdateNever
}

func desiredAndUndesiredInstances(
	instances []instance.Description, settings groupSettings) ([]instance.Description, []instance.Description) {

	desiredHash := settings.config.InstanceHash()
	desired := []instance.Description{}
	undesired := []instance.Description{}

	for _, inst := range instances {

		actualConfig, specified := inst.Tags[group.ConfigSHATag]
		if specified && actualConfig == desiredHash {
			desired = append(desired, inst)
		} else if canDestroy(inst, settings) {
			undesired = append(undesired, inst)
		}
	}

	return desired, undesired
}

type rollingupdate struct {
	desc         string
	scaled       Scaled
	updatingFrom groupSettings
	updatingTo   groupSettings
	stop         chan bool
}

func (r rollingupdate) Explain() string {
	return r.desc
}

func (r *rollingupdate) waitUntilQuiesced(pollInterval time.Duration, expectedNewInstances int) error {
	// Block until the expected number of instances in the desired state are ready.  Updates are unconcerned with
	// the health of instances in the undesired state.  This allows a user to dig out of a hole where the original
	// state of the group is bad, and instances are not reporting as healthy.
	log.Info("waitUntilQuiesced", "expectedNewInstances", expectedNewInstances)
	// TODO: start processing right away instead of waiting for first tick
	ticker := time.NewTicker(pollInterval)
	for {
		select {
		case <-ticker.C:
			// Gather instances in the scaler with the desired state
			// Check:
			//   - that the scaler has the expected number of instances
			//   - instances with the desired config are healthy

			// TODO(wfarner): Get this information from the scaler to reduce redundant network calls.
			instances, err := labelAndList(r.scaled)
			if err != nil {
				return err
			}

			// The update is only concerned with instances being created in the course of the update.
			// The health of instances in any other state is irrelevant.  This behavior is important
			// especially if the previous state of the group is unhealthy and the update is attempting to
			// restore health.
			matching, _ := desiredAndUndesiredInstances(instances, r.updatingTo)
			log.Info("waitUntilQuiesced", "totalInstances", len(instances), "matchingInstances", len(matching))

			// Now that we have isolated the instances with the new configuration, check if they are all
			// healthy.  We do not consider an update successful until the target number of instances are
			// confirmed healthy.
			// The following design choices are currently implemented:
			//
			//   - the update will continue indefinitely if one or more instances are in the
			//     flavor.UnknownHealth state.  Operators must stop the update and diagnose the cause.
			//
			//   - the update is stopped immediately if any instance enters the flavor.Unhealthy state.
			//
			//   - the update will proceed with other instances immediately when the currently-expected
			//     number of instances are observed in the flavor.Healthy state.
			//
			numHealthy := 0
			for _, inst := range matching {
				// TODO(wfarner): More careful thought is needed with respect to blocking and timeouts
				// here.  This might mean formalizing timeout behavior for different types of RPCs in
				// the group, and/or documenting the expectations for plugin implementations.
				switch r.scaled.Health(inst) {
				case flavor.Healthy:
					log.Info("waitUntilQuiesced", "health", "heathy", "nodeID", inst.ID)
					numHealthy++
				case flavor.Unhealthy:
					log.Error("waitUntilQuiesced", "health", "unheathy", "nodeID", inst.ID)
					return fmt.Errorf("Instance %s is unhealthy", inst.ID)
				case flavor.Unknown:
					log.Info("waitUntilQuiesced", "health", "unknown", "nodeID", inst.ID)
				}
			}

			if numHealthy >= int(expectedNewInstances) {
				log.Info("waitUntilQuiesced",
					"msg", "Scaler has quiesced, terminating loop",
					"numHealthy", numHealthy,
					"expectedNewInstances", expectedNewInstances)
				return nil
			}
			log.Info("waitUntilQuiesced",
				"msg", "Waiting for scaler to quiesce",
				"numHealthy", numHealthy,
				"expectedNewInstances", expectedNewInstances)

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

	instances, err := labelAndList(r.scaled)
	if err != nil {
		return err
	}

	// First determine if any new instances should be created
	desired, _ := desiredAndUndesiredInstances(instances, r.updatingTo)
	expectedNewInstances := len(desired)
	log.Info("RollingUpdate-Run", "expectedNewInstances", expectedNewInstances)

	for {
		// Wait until any new nodes are healthy
		desiredSize := len(r.updatingTo.config.Allocation.LogicalIDs)
		if desiredSize == 0 {
			desiredSize = int(r.updatingTo.config.Allocation.Size)
		}
		err := r.waitUntilQuiesced(pollInterval, minInt(expectedNewInstances, desiredSize))
		if err != nil {
			return err
		}

		instances, err := labelAndList(r.scaled)
		if err != nil {
			return err
		}

		// Now check if we have instances that do not match the hash
		_, undesiredInstances := desiredAndUndesiredInstances(instances, r.updatingTo)
		log.Info("RollingUpdate-Run", "undesiredInstances", len(undesiredInstances))
		if len(undesiredInstances) == 0 {
			break
		}

		// Sort instances first to ensure predictable destroy order.
		sort.Sort(sortByID{list: undesiredInstances, settings: &r.updatingFrom})

		// TODO(wfarner): Make the 'batch size' configurable.
		if canDestroy(undesiredInstances[0], r.updatingFrom) {
			// we do not self-destruct in any cases.
			if err := r.scaled.Destroy(undesiredInstances[0], instance.RollingUpdate); err != nil {
				log.Warn("Failed to destroy instance during rolling update", "ID", undesiredInstances[0].ID, "err", err)
				return err
			}
		}

		// Increment new instance count to replace the node that was just destroyed
		expectedNewInstances++
	}

	return nil
}

func (r *rollingupdate) Stop() {
	close(r.stop)
}
