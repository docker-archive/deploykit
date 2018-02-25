package enrollment

import (
	"time"

	enrollment "github.com/docker/infrakit/pkg/controller/enrollment/types"
	"github.com/docker/infrakit/pkg/controller/internal"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log     = logutil.New("module", "controller/enrollment")
	debugV  = logutil.V(200)
	debugV2 = logutil.V(500)

	// DefaultOptions return an Options with default values filled in.
	DefaultOptions = enrollment.Options{
		SyncInterval:             types.Duration(5 * time.Second),
		DestroyOnTerminate:       false,
		SourceParseErrPolicy:     enrollment.SourceParseErrorEnableDestroy,
		EnrollmentParseErrPolicy: enrollment.EnrolledParseErrorEnableProvision,
	}
)

// NewController returns a controller implementation
func NewController(scope scope.Scope, options enrollment.Options) controller.Controller {
	return internal.NewController(
		// the constructor
		func(spec types.Spec) (internal.Managed, error) {
			return newEnroller(scope, options)
		},
		// the key function
		func(metadata types.Metadata) string {
			return metadata.Name
		},
	)
}

// NewTypedControllers return typed controllers
func NewTypedControllers(scope scope.Scope,
	options enrollment.Options) func() (map[string]controller.Controller, error) {

	return (internal.NewController(
		// the constructor
		func(spec types.Spec) (internal.Managed, error) {
			log.Debug("Creating managed object", "spec", spec)
			return newEnroller(scope, options)
		},
		// the key function
		func(metadata types.Metadata) string {
			return metadata.Name
		},
	)).Controllers
}

func (l *enroller) started() bool {
	l.lock.RLock()
	defer l.lock.RUnlock()

	return l.running
}
