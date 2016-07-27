package swarm

import "time"

// Swarm represents a swarm.
type Swarm struct {
	ID string
	Meta
	Spec       Spec
	JoinTokens JoinTokens
}

// JoinTokens contains the tokens workers and managers need to join the swarm.
type JoinTokens struct {
	Worker  string
	Manager string
}

// Spec represents the spec of a swarm.
type Spec struct {
	Annotations

	Orchestration OrchestrationConfig `json:",omitempty"`
	Raft          RaftConfig          `json:",omitempty"`
	Dispatcher    DispatcherConfig    `json:",omitempty"`
	CAConfig      CAConfig            `json:",omitempty"`
	TaskDefaults  TaskDefaults        `json:",omitempty"`
}

// OrchestrationConfig represents orchestration configuration.
type OrchestrationConfig struct {
	TaskHistoryRetentionLimit int64 `json:",omitempty"`
}

// TaskDefaults parameterizes cluster-level task creation with default values.
type TaskDefaults struct {
	// LogDriver selects the log driver to use for tasks created in the
	// orchestrator if unspecified by a service.
	//
	// Updating this value will only have an affect on new tasks. Old tasks
	// will continue use their previously configured log driver until
	// recreated.
	LogDriver *Driver `json:",omitempty"`
}

// RaftConfig represents raft configuration.
type RaftConfig struct {
	SnapshotInterval           uint64 `json:",omitempty"`
	KeepOldSnapshots           uint64 `json:",omitempty"`
	LogEntriesForSlowFollowers uint64 `json:",omitempty"`
	HeartbeatTick              uint32 `json:",omitempty"`
	ElectionTick               uint32 `json:",omitempty"`
}

// DispatcherConfig represents dispatcher configuration.
type DispatcherConfig struct {
	HeartbeatPeriod uint64 `json:",omitempty"`
}

// CAConfig represents CA configuration.
type CAConfig struct {
	NodeCertExpiry time.Duration `json:",omitempty"`
	ExternalCAs    []*ExternalCA `json:",omitempty"`
}

// ExternalCAProtocol represents type of external CA.
type ExternalCAProtocol string

// ExternalCAProtocolCFSSL CFSSL
const ExternalCAProtocolCFSSL ExternalCAProtocol = "cfssl"

// ExternalCA defines external CA to be used by the cluster.
type ExternalCA struct {
	Protocol ExternalCAProtocol
	URL      string
	Options  map[string]string `json:",omitempty"`
}

// InitRequest is the request used to init a swarm.
type InitRequest struct {
	ListenAddr      string
	AdvertiseAddr   string
	ForceNewCluster bool
	Spec            Spec
}

// JoinRequest is the request used to join a swarm.
type JoinRequest struct {
	ListenAddr    string
	AdvertiseAddr string
	RemoteAddrs   []string
	JoinToken     string // accept by secret
}

// LocalNodeState represents the state of the local node.
type LocalNodeState string

const (
	// LocalNodeStateInactive INACTIVE
	LocalNodeStateInactive LocalNodeState = "inactive"
	// LocalNodeStatePending PENDING
	LocalNodeStatePending LocalNodeState = "pending"
	// LocalNodeStateActive ACTIVE
	LocalNodeStateActive LocalNodeState = "active"
	// LocalNodeStateError ERROR
	LocalNodeStateError LocalNodeState = "error"
)

// Info represents generic information about swarm.
type Info struct {
	NodeID   string
	NodeAddr string

	LocalNodeState   LocalNodeState
	ControlAvailable bool
	Error            string

	RemoteManagers []Peer
	Nodes          int
	Managers       int

	Cluster Swarm
}

// Peer represents a peer.
type Peer struct {
	NodeID string
	Addr   string
}

// UpdateFlags contains flags for SwarmUpdate.
type UpdateFlags struct {
	RotateWorkerToken  bool
	RotateManagerToken bool
}
