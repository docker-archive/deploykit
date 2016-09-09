package scaler

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
	"sort"
	"time"
)

type rollingupdate struct {
	context  *groupContext
	newProps groupProperties
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func planRollingUpdate(id group.ID, context *groupContext, newProps groupProperties) (updatePlan, error) {

	instances, err := context.scaled.describe()
	if err != nil {
		return updatePlan{}, err
	}

	undesiredInstances, err := findUndesiredInstances(
		instances,
		instanceConfigHash(newProps.InstancePluginProperties))
	if err != nil {
		return updatePlan{}, err
	}

	sizeChange := int(newProps.Size) - int(context.properties.Size)
	rollCount := minInt(len(undesiredInstances), int(newProps.Size))

	if rollCount == 0 && sizeChange == 0 {
		return updatePlan{desc: "Noop", execute: func() error { return nil }}, nil
	}

	rollDesc := fmt.Sprintf("a rolling update on %d instances", rollCount)

	var desc string
	switch {
	case sizeChange == 0:
		desc = fmt.Sprintf("Performs %s", rollDesc)
	case sizeChange < 0:
		if rollCount > 0 {
			desc = fmt.Sprintf(
				"Terminates %d instances to reduce the group size to %d, then performs %s",
				int(sizeChange)*-1,
				newProps.Size,
				rollDesc)
		} else {
			desc = fmt.Sprintf(
				"Terminates %d instances to reduce the group size to %d",
				int(sizeChange)*-1,
				newProps.Size)
		}

	case sizeChange > 0:
		if rollCount > 0 {
			desc = fmt.Sprintf(
				"Performs %s, then adds %d instances to increase the group size to %d",
				rollDesc,
				sizeChange,
				newProps.Size)
		} else {
			desc = fmt.Sprintf(
				"Adds %d instances to increase the group size to %d",
				sizeChange,
				newProps.Size)
		}
	}

	execute := func() error {
		update := rollingupdate{context: context, newProps: newProps}
		update.Run()
		log.Infof("Finished updating group %s", id)

		return nil
	}

	return updatePlan{desc: desc, execute: execute}, nil
}

func findUndesiredInstances(instances []instance.Description, desiredConfig string) ([]instance.ID, error) {
	undesiredInstances := []instance.ID{}
	for _, inst := range instances {
		actualConfig, specified := inst.Tags[configTag]
		if !specified || actualConfig != desiredConfig {
			undesiredInstances = append(undesiredInstances, inst.ID)
		}
	}

	return undesiredInstances, nil
}

// Run identifies instances not matching the desired state and destroys them one at a time until all instances in the
// group match the desired state, with the desired number of instances.
func (r *rollingupdate) Run() error {
	originalSize := r.context.properties.Size

	// If the number of instances is being decreased, first lower the group size.  This eliminates
	// instances that would otherwise be rolled first, avoiding unnecessary work.
	// We could further optimize by selecting undesired instances to destroy, for example if the
	// scaler already has a mix of desired and undesired instances.
	if originalSize > r.newProps.Size {
		r.context.scaler.SetSize(r.newProps.Size)
	}

	r.context.setProperties(&r.newProps)
	r.context.scaled.setProvisionTemplate(r.newProps.InstancePluginProperties, identityTags(r.newProps))

	for {
		// TODO(wfarner): Wait until all desired instances in the new configuration are healthy.
		time.Sleep(1 * time.Second)

		instances, err := r.context.scaled.describe()
		if err != nil {
			return err
		}

		undesiredInstances, err := findUndesiredInstances(
			instances,
			instanceConfigHash(r.newProps.InstancePluginProperties))
		if err != nil {
			return err
		}

		if len(undesiredInstances) == 0 {
			break
		}

		log.Infof("Found %d undesired instances", len(undesiredInstances))

		// Sort instances first to ensure predictable destroy order.
		sort.Sort(sortByID(undesiredInstances))

		err = r.context.scaled.Destroy(undesiredInstances[0])
		if err != nil {
			return err
		}
	}

	// Rolling has completed.  If the update included a group size increase, perform that now.
	if originalSize < r.newProps.Size {
		r.context.scaler.SetSize(r.newProps.Size)
	}

	return nil
}
