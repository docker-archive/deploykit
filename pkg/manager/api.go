package manager

import (
	"github.com/docker/infrakit/pkg/leader"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/controller"
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
	debugV3 = logutil.V(550)
)

// Backend is the admin / server interface
type Backend interface {
	stack.Interface

	Controllers() (map[string]controller.Controller, error)
	Groups() (map[group.ID]group.Plugin, error)
	Metadata() (map[string]metadata.Plugin, error)

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

	// Controllers are the names of the controllers this manager manages
	Controllers []plugin.Name

	// Plugins for plugin lookup
	//Plugins func() discovery.Plugins `json:"-" yaml:"-"`

	// Leader is the leader detector
	Leader leader.Detector `json:"-" yaml:"-"`

	// LeaderStore persists leadership information
	LeaderStore leader.Store `json:"-" yaml:"-"`

	// SpecStore persists user specs
	SpecStore store.Snapshot `json:"-" yaml:"-"`

	// MetadataStore persists var information
	MetadataStore store.Snapshot `json:"-" yaml:"-"`

	// LeaderCommitSpecsRetries is how many times to retry commit specs when becomes leader
	LeaderCommitSpecsRetries int

	// LeaderCommitSpecsRetryInterval is how long to wait before next retry
	LeaderCommitSpecsRetryInterval types.Duration
}
