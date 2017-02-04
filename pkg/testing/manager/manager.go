package manager

// Plugin implements manager.Manager
type Plugin struct {

	// DoIsLeader returns true if manager is leader
	DoIsLeader func() (bool, error)
}

// IsLeader returns true if manager is leader
func (t *Plugin) IsLeader() (bool, error) {
	return t.DoIsLeader()
}
