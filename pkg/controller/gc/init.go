package gc

import (
	"fmt"
	"sync"

	"github.com/docker/infrakit/pkg/fsm"
	"github.com/docker/infrakit/pkg/spi/instance"
)

var (
	models = map[string]func() (Model, error){}
	lock   sync.RWMutex
)

// Register registers an available model
func Register(key string, builder func() (Model, error)) {
	lock.Lock()
	defer lock.Unlock()

	models[key] = builder
}

func model(key string) (Model, error) {
	lock.RLock()
	defer lock.RUnlock()

	f, has := models[key]

	if !has {
		return nil, fmt.Errorf("unknow model %v", key)
	}
	return f()
}

// Model is the interace
type Model interface {
	Start()
	Stop()
	Spec() *fsm.Spec
	New() fsm.Instance
	FoundNode(fsm.Instance, instance.Description)
	LostNode(fsm.Instance)
	FoundInstance(fsm.Instance, instance.Description)
	LostInstance(fsm.Instance)
}
