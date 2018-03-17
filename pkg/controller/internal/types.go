package internal

import (
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log = logutil.New("module", "controller/internal")

	// this is lower level, with 1 level up (the actual controllers), so give a scaling factor for the verbosity level
	debugV  = logutil.V(2 * 500)
	debugV2 = logutil.V(2 * 1000)
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

	CurrentSpec() types.Spec
	SetPrevSpec(types.Spec)
	GetPrevSpec() *types.Spec // optional since the first one will not have it set

	Metadata() metadata.Plugin
	Events() event.Plugin

	Plan(controller.Operation, types.Spec) (*types.Object, *controller.Plan, error)
	Enforce(types.Spec) (*types.Object, error)
	Inspect() (*types.Object, error)
	Free() (*types.Object, error)
	Terminate() (*types.Object, error)
}
