package manager

import (
	"net/url"

	"github.com/docker/infrakit/pkg/controller"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/leader"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log = logutil.New("module", "manager")

	debugV  = logutil.V(100)
	debugV2 = logutil.V(500)

	// InterfaceSpec is the current name and version of the Instance API.
	InterfaceSpec = spi.InterfaceSpec{
		Name:    "Manager",
		Version: "0.1.0",
	}
)

// Leadership is the interface for getting information about the current leader node
type Leadership interface {
	// IsLeader returns true only if for certain this is a leader. False if not or unknown.
	IsLeader() (bool, error)

	// LeaderLocation returns the location of the leader
	LeaderLocation() (*url.URL, error)
}

// Manager is the interface for interacting locally or remotely with the manager
type Manager interface {
	Leadership
	stack.Interface
}

// Backend is the admin / server interface
type Backend interface {
	// group.Plugin

	// metadata.Updatable

	Controllers() (map[string]controller.Controller, error)
	Groups() (map[group.ID]group.Plugin, error)
	Metadata() (map[string]metadata.Plugin, error)

	Manager
	Start() (<-chan struct{}, error)
	Stop()
}

// Options capture the options for starting up the plugin.
type Options struct {

	// Name the manager runs as
	Name plugin.Name

	// Name of the Group plugin
	Group plugin.Name

	// Name of the Metadata plugin. Must be pointing to Updatable
	Metadata plugin.Name

	// Plugins for plugin lookup
	Plugins func() discovery.Plugins `json:"-" yaml:"-"`

	// Leader is the leader detector
	Leader leader.Detector `json:"-" yaml:"-"`

	// LeaderStore persists leadership information
	LeaderStore leader.Store `json:"-" yaml:"-"`

	// SpecStore persists user specs
	SpecStore store.Snapshot `json:"-" yaml:"-"`

	// MetadataStore persists var information
	MetadataStore store.Snapshot `json:"-" yaml:"-"`

	// MetadataRefreshInterval is the interval to check for updates to metadata
	MetadataRefreshInterval types.Duration

	// LeaderCommitSpecsRetries is how many times to retry commit specs when becomes leader
	LeaderCommitSpecsRetries int

	// LeaderCommitSpecsRetryInterval is how long to wait before next retry
	LeaderCommitSpecsRetryInterval types.Duration
}
