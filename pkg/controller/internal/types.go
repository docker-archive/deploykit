package internal

import (
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log     = logutil.New("module", "controller/internal")
	debugV  = logutil.V(500)
	debugV2 = logutil.V(900)
)

// ControlLoop gives status and means to stop the object
type ControlLoop interface {
	Start()
	Running() bool
	Stop() error
}

// Managed is the interface implemented by managed objects within a controller
type Managed interface {
	ControlLoop

	Metadata() metadata.Plugin

	Plan(controller.Operation, types.Spec) (*types.Object, *controller.Plan, error)
	Enforce(types.Spec) (*types.Object, error)
	Inspect() (*types.Object, error)
	Free() (*types.Object, error)
	Terminate() (*types.Object, error)
}
