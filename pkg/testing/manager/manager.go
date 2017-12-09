package manager

import (
	"net/url"

	"github.com/docker/infrakit/pkg/types"
)

// Plugin implements manager.Manager
type Plugin struct {

	// DoLeaderLocation returns the location
	DoLeaderLocation func() (*url.URL, error)

	// DoIsLeader returns true if manager is leader
	DoIsLeader func() (bool, error)

	// DoEnforce enforces infrastructure state to match that of the specs
	DoEnforce func(specs []types.Spec) error

	// DoSpecs returns the current state of the infrastructure
	DoSpecs func() ([]types.Spec, error)

	// DoInspect returns the current state of the infrastructure
	DoInspect func() ([]types.Object, error)

	// DoTerminate destroys all resources associated with the specs
	DoTerminate func(specs []types.Spec) error
}

// IsLeader returns true if manager is leader
func (t *Plugin) IsLeader() (bool, error) {
	return t.DoIsLeader()
}

// LeaderLocation returns the location of the leader
func (t *Plugin) LeaderLocation() (*url.URL, error) {
	return t.DoLeaderLocation()
}

// Enforce enforces infrastructure state to match that of the specs
func (t *Plugin) Enforce(specs []types.Spec) error {
	return t.DoEnforce(specs)
}

// Specs returns the current specs being enforced
func (t *Plugin) Specs() ([]types.Spec, error) {
	return t.DoSpecs()
}

// Inspect returns the current state of the infrastructure
func (t *Plugin) Inspect() ([]types.Object, error) {
	return t.DoInspect()
}

// Terminate destroys all resources associated with the specs
func (t *Plugin) Terminate(specs []types.Spec) error {
	return t.DoTerminate(specs)
}
