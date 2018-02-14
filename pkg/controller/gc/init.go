package gc

import (
	"fmt"
	"sync"

	"github.com/docker/infrakit/pkg/fsm"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

var (
	models = map[string]func(*types.Any) (Model, error){}
	lock   sync.RWMutex
)

// Register registers an available model
func Register(key string, builder func(*types.Any) (Model, error)) {
	lock.Lock()
	defer lock.Unlock()

	models[key] = builder
}

func model(key string, modelProperties *types.Any) (Model, error) {
	lock.RLock()
	defer lock.RUnlock()

	f, has := models[key]

	if !has {
		return nil, fmt.Errorf("unknow model %v", key)
	}
	return f(modelProperties)
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
	New() fsm.Instance
	FoundNode(fsm.Instance, instance.Description) error
	LostNode(fsm.Instance)
	FoundInstance(fsm.Instance, instance.Description) error
	LostInstance(fsm.Instance)
	GCNode() <-chan fsm.Instance
	GCInstance() <-chan fsm.Instance
}
