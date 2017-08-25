package simulator

import (
	"fmt"
	"sync"
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/store/file"
	"github.com/docker/infrakit/pkg/store/mem"
	"github.com/docker/infrakit/pkg/types"
)

var instanceLogger = logutil.New("module", "simulator/instance")

const (
	debugV = logutil.V(500)
)

// NewInstance returns a typed instance plugin
func NewInstance(name string, options Options) instance.Plugin {
	l := &instanceSimulator{
		name: name,
	}

	switch options.Store {
	case StoreFile:
		l.instances = file.NewStore(name, options.Dir)
	case StoreMem:
		l.instances = mem.NewStore(name)
	}
	return l
}

type instanceSimulator struct {
	name      string
	instances store.KV
	lock      sync.Mutex
}

// Validate performs local validation on a provision request.
func (s *instanceSimulator) Validate(req *types.Any) error {
	instanceLogger.Debug("Validate", "req", req)
	return nil
}

// Provision creates a new instance based on the spec.
func (s *instanceSimulator) Provision(spec instance.Spec) (*instance.ID, error) {
	instanceLogger.Debug("Provision", "name", s.name, "spec", spec, "V", debugV)
	s.lock.Lock()
	defer s.lock.Unlock()
	key := fmt.Sprintf("%v", time.Now().UnixNano())
	description := instance.Description{
		ID:         instance.ID(key),
		Tags:       spec.Tags,
		LogicalID:  spec.LogicalID,
		Properties: types.AnyValueMust(spec),
	}
	buff, err := types.AnyValueMust(description).MarshalYAML()
	if err != nil {
		return nil, err
	}

	err = s.instances.Write(description.ID, buff)
	instanceLogger.Debug("Provisioned", "id", description.ID, "spec", spec, "err", err)
	return &description.ID, err
}

// Label labels the instance
func (s *instanceSimulator) Label(key instance.ID, labels map[string]string) error {
	instanceLogger.Debug("Label", "name", s.name, "instance", key, "labels", labels, "V", debugV)
	s.lock.Lock()
	defer s.lock.Unlock()

	exists, err := s.instances.Exists(key)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("not found %v", key)
	}

	buff, err := s.instances.Read(key)
	if err != nil {
		return err
	}

	n := instance.Description{}
	if err := types.AnyYAMLMust(buff).Decode(&n); err != nil {
		return err
	}
	if n.Tags == nil {
		n.Tags = map[string]string{}
	}

	for k, v := range labels {
		n.Tags[k] = v
	}

	buff, err = types.AnyValueMust(n).MarshalYAML()
	if err != nil {
		return err
	}

	return s.instances.Write(key, buff)
}

// Destroy terminates an existing instance.
func (s *instanceSimulator) Destroy(instance instance.ID, context instance.Context) error {
	instanceLogger.Debug("Destroy", "name", s.name, "instance", instance, "context", context, "V", debugV)
	s.lock.Lock()
	defer s.lock.Unlock()

	exists, err := s.instances.Exists(instance)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("not found %v", instance)
	}
	return s.instances.Delete(instance)
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
// The properties flag indicates the client is interested in receiving details about each instance.
func (s *instanceSimulator) DescribeInstances(labels map[string]string,
	properties bool) ([]instance.Description, error) {
	instanceLogger.Debug("DescribeInstances", "name", s.name, "labels", labels, "V", debugV)
	s.lock.Lock()
	defer s.lock.Unlock()

	matches := []instance.Description{}

	err := store.Visit(s.instances,
		labels,
		func(v interface{}) map[string]string {
			return v.(instance.Description).Tags
		},
		func(buff []byte) (interface{}, error) {
			desc := instance.Description{}
			err := types.AnyYAMLMust(buff).Decode(&desc)
			return desc, err
		},
		func(v interface{}) (bool, error) {
			matches = append(matches, v.(instance.Description))
			return true, nil
		})

	return matches, err
}
