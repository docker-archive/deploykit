package gc

import (
	"fmt"
	"sync"

	gc "github.com/docker/infrakit/pkg/controller/gc/types"
	"github.com/docker/infrakit/pkg/fsm"
	"github.com/docker/infrakit/pkg/spi/instance"
)

var (
	models = map[string]func(gc.Properties) (Model, error){}
	lock   sync.RWMutex
)

// Register registers an available model
func Register(key string, builder func(gc.Properties) (Model, error)) {
	lock.Lock()
	defer lock.Unlock()

	models[key] = builder
}

func model(properties gc.Properties) (Model, error) {
	lock.RLock()
	defer lock.RUnlock()

	f, has := models[properties.Model]
	if !has {
		return nil, fmt.Errorf("unknow model %v", properties.Model)
	}
	return f(properties)
}

// Model is the interface offered by the workflow model.  In general, we consider two sides:
// Node and Instance.  Node side is the engine/runtime running on Instance side.  Because they
// are resources representing a conceptual entity, yet with disconnected lifecycles, there will
// be cases when the link is lost and we need to perform cleanup to ensure a consistent, operational
// view of the cluster.
type Model interface {
	Start()
	Stop()
	Spec() *fsm.Spec
	New() fsm.FSM
	FoundNode(fsm.FSM, instance.Description) error
	LostNode(fsm.FSM)
	FoundInstance(fsm.FSM, instance.Description) error
	LostInstance(fsm.FSM)
	GCNode() <-chan fsm.FSM
	GCInstance() <-chan fsm.FSM
}
