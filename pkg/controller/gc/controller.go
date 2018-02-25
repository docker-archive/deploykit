package gc

import (
	"time"

	gc "github.com/docker/infrakit/pkg/controller/gc/types"
	"github.com/docker/infrakit/pkg/controller/internal"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log     = logutil.New("module", "controller/gc")
	debugV  = logutil.V(200)
	debugV2 = logutil.V(500)

	// DefaultOptions return an Options with default values filled in.
	DefaultOptions = gc.Options{
		PluginRetryInterval: types.Duration(1 * time.Second),
	}
)

// NewController returns a controller implementation
func NewController(scope scope.Scope, options gc.Options) func() (map[string]controller.Controller, error) {

	return (internal.NewController(
		// the constructor
		func(spec types.Spec) (internal.Managed, error) {
			return newReaper(scope, options)
		},
		// the key function
		func(metadata types.Metadata) string {
			return metadata.Name
		},
	)).Controllers
}
