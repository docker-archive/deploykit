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

	// Enforce enforces infrastructure state to match that of the specs
	Enforce(specs []types.Spec) error

	// Inspect returns the current state of the infrastructure
	Inspect() ([]types.Object, error)

	// Terminate destroys all resources associated with the specs
	Terminate(specs []types.Spec) error
}

// Backend is the admin / server interface
type Backend interface {
	group.Plugin

	metadata.Updatable

	Controllers() (map[string]controller.Controller, error)
	Groups() (map[group.ID]group.Plugin, error)

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
	Leader leader.Detector

	// LeaderStore persists leadership information
	LeaderStore leader.Store

	// SpecStore persists user specs
	SpecStore store.Snapshot

	// MetadataStore persists var information
	MetadataStore store.Snapshot
}
