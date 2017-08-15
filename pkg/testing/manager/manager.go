package manager

import (
	"net/url"
)

// Plugin implements manager.Manager
type Plugin struct {

	// DoLeaderLocation returns the location
	DoLeaderLocation func() (*url.URL, error)

	// DoIsLeader returns true if manager is leader
	DoIsLeader func() (bool, error)
}

// IsLeader returns true if manager is leader
func (t *Plugin) IsLeader() (bool, error) {
	return t.DoIsLeader()
}

// LeaderLocation returns the location of the leader
func (t *Plugin) LeaderLocation() (*url.URL, error) {
	return t.DoLeaderLocation()
}
