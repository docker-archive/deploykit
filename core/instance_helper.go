package core

import (
	"strings"

	"github.com/ryanuber/go-glob"
)

// InstanceFilter creates a filter for the data (conditional AND)
// If filter is empty, everything is returned
type InstanceFilter struct {
	LifeCycleState string
	DisplayName    string
}

// filterInstances returns a filtered list of instances based on the filter provided
func filterInstances(instances []Instance, filter InstanceFilter) []Instance {
	finalInstances := instances[:0]
	for _, instance := range instances {
		conditional := true
		if filter.LifeCycleState != "" {
			conditional = conditional && strings.ToUpper(filter.LifeCycleState) == instance.LifeCycleState
		}
		if filter.DisplayName != "" {
			conditional = conditional && glob.Glob(filter.DisplayName, instance.DisplayName)
		}
		if conditional {
			finalInstances = append(finalInstances, instance)
		}
	}
	return finalInstances
}
