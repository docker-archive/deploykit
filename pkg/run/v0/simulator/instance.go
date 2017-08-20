package simulator

import (
	"fmt"
	"sync"
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/store/file"
	"github.com/docker/infrakit/pkg/types"
)

var instanceLogger = logutil.New("module", "simulator/instance")

// NewInstance returns a typed instance plugin
func NewInstance(name, dir string) instance.Plugin {
	l := &instanceSimulator{
		name:      name,
		instances: file.NewStore(name, dir, true).Init(),
	}
	return l
}

type instanceSimulator struct {
	name      string
	instances *file.Store
	lock      sync.Mutex
}

// Validate performs local validation on a provision request.
func (s *instanceSimulator) Validate(req *types.Any) error {
	instanceLogger.Info("Validate", "req", req)
	return nil
}

// Provision creates a new instance based on the spec.
func (s *instanceSimulator) Provision(spec instance.Spec) (*instance.ID, error) {
	instanceLogger.Info("Provision", "name", s.name, "spec", spec)
	s.lock.Lock()
	defer s.lock.Unlock()
	key := time.Now().UnixNano()
	description := instance.Description{
		ID:         instance.ID(key),
		Tags:       spec.Tags,
		LogicalID:  spec.LogicalID,
		Properties: types.AnyValueMust(spec),
	}
	err := s.instances.Write(description.ID, description)
	return &description.ID, err
}

// Label labels the instance
func (s *instanceSimulator) Label(key instance.ID, labels map[string]string) error {
	instanceLogger.Info("Label", "name", s.name, "instance", key, "labels", labels)
	s.lock.Lock()
	defer s.lock.Unlock()

	exists, err := s.instances.Exists(key)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("not found %v", key)
	}

	v, err := s.instances.Read(key, func(buff []byte) (interface{}, error) {
		d := instance.Description{}
		err := types.AnyYAMLMust(buff).Decode(&d)
		return d, err
	})
	if err != nil {
		return err
	}

	n := v.(instance.Description)
	if n.Tags == nil {
		n.Tags = map[string]string{}
	}

	for k, v := range labels {
		n.Tags[k] = v
	}
	return s.instances.Write(key, n)
}

// Destroy terminates an existing instance.
func (s *instanceSimulator) Destroy(instance instance.ID, context instance.Context) error {
	instanceLogger.Info("Destroy", "name", s.name, "instance", instance, "context", context)
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
	instanceLogger.Info("DescribeInstances", "name", s.name, "labels", labels)
	s.lock.Lock()
	defer s.lock.Unlock()

	matches := []instance.Description{}

	err := s.instances.All(labels,
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
