package simulator

import (
	"fmt"
	"sync"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

var instanceLogger = logutil.New("module", "simulator/instance")

type instanceSimulator struct {
	name      string
	instances map[instance.ID]instance.Description
	lock      sync.Mutex
}

func (s *instanceSimulator) alloc() *instanceSimulator {
	s.instances = map[instance.ID]instance.Description{}
	return s
}

// Validate performs local validation on a provision request.
func (s *instanceSimulator) Validate(req *types.Any) error {
	return nil
}

// Provision creates a new instance based on the spec.
func (s *instanceSimulator) Provision(spec instance.Spec) (*instance.ID, error) {
	instanceLogger.Info("Provision", "name", s.name, "spec", spec)
	s.lock.Lock()
	defer s.lock.Unlock()
	description := instance.Description{
		ID:         instance.ID(fmt.Sprintf("i-%d", len(s.instances))),
		Tags:       spec.Tags,
		LogicalID:  spec.LogicalID,
		Properties: types.AnyValueMust(spec),
	}
	s.instances[description.ID] = description
	return &description.ID, nil
}

// Label labels the instance
func (s *instanceSimulator) Label(instance instance.ID, labels map[string]string) error {
	instanceLogger.Info("Label", "name", s.name, "instance", instance, "labels", labels)
	s.lock.Lock()
	defer s.lock.Unlock()

	n, has := s.instances[instance]
	if !has {
		return fmt.Errorf("not found %v", instance)
	}

	if n.Tags == nil {
		n.Tags = map[string]string{}
	}

	for k, v := range labels {
		n.Tags[k] = v
	}
	return nil
}

// Destroy terminates an existing instance.
func (s *instanceSimulator) Destroy(instance instance.ID, context instance.Context) error {
	instanceLogger.Info("Destroy", "name", s.name, "instance", instance, "context", context)
	s.lock.Lock()
	defer s.lock.Unlock()

	_, has := s.instances[instance]
	if !has {
		return fmt.Errorf("not found %v", instance)
	}

	delete(s.instances, instance)
	return nil
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
// The properties flag indicates the client is interested in receiving details about each instance.
func (s *instanceSimulator) DescribeInstances(labels map[string]string,
	properties bool) ([]instance.Description, error) {
	instanceLogger.Info("DescribeInstances", "name", s.name, "labels", labels)
	s.lock.Lock()
	defer s.lock.Unlock()

	matches := []instance.Description{}
	for _, v := range s.instances {

		if hasDifferentTag(labels, v.Tags) {
			continue
		}
		matches = append(matches, v)
	}
	return matches, nil
}

func hasDifferentTag(expected, actual map[string]string) bool {
	if len(actual) == 0 {
		return true
	}
	for k, v := range expected {
		if a, ok := actual[k]; ok && a != v {
			return true
		}
	}

	return false
}
