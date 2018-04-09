package simulator

import (
	"fmt"
	"sync"
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/store/file"
	"github.com/docker/infrakit/pkg/store/mem"
	"github.com/docker/infrakit/pkg/types"
)

var instanceLogger = logutil.New("module", "simulator/instance")

const (
	debugV  = logutil.V(500)
	debugV2 = logutil.V(1000)
)

// NewInstance returns a typed instance plugin
func NewInstance(pname plugin.Name, name string, options Options) instance.Plugin {
	l := &instanceSimulator{
		plugin:  pname,
		name:    name,
		options: options,
	}

	switch options.Store {
	case StoreFile:
		l.instances = file.NewStore(name, options.Dir)
	case StoreMem:
		l.instances = mem.NewStore(name)
	}

	log.Info("Simulator starting", "delay", options.StartDelay, "name", name, "plugin", pname)
	go func() {
		// Intentionally hold the lock for the duration of
		// the delay to make the plugin unavailable

		l.lock.Lock()
		defer l.lock.Unlock()

		delay := time.After(options.StartDelay)
		for {
			select {
			case <-delay:
				instanceLogger.Info("Delay done. Continue", "name", name, "plugin", pname)
				return
			case <-time.Tick(1 * time.Second):
				instanceLogger.Info("Simulator starting up.", "name", name, "plugin", pname)
			}
		}
	}()

	return l
}

type instanceSimulator struct {
	plugin    plugin.Name
	name      string
	instances store.KV
	lock      sync.RWMutex
	options   Options
}

// Validate performs local validation on a provision request.
func (s *instanceSimulator) Validate(req *types.Any) error {
	instanceLogger.Debug("Validate", "req", req, "plugin", s.plugin)
	return nil
}

// Provision creates a new instance based on the spec.
func (s *instanceSimulator) Provision(spec instance.Spec) (*instance.ID, error) {
	instanceLogger.Debug("Provision", "name", s.name, "spec", spec, "V", debugV, "plugin", s.plugin,
		"method", "Provision")

	<-time.After(s.options.ProvisionDelay)

	// simulator feature....
	control := struct {
		Cap   int            `json:"simulator_cap" yaml:"simulator_cap"`
		Delay types.Duration `json:"simulator_delay" yaml:"simulator_delay"`
	}{}

	err := spec.Properties.Decode(&control)
	if err != nil {
		instanceLogger.Error("Error decoding simulator parameters", "err", err)
	} else {
		instanceLogger.Info("Simulator has control parameters", "control", control)
		if control.Cap > 0 {
			found, err := s.describeInstances(map[string]string{}, false)
			instanceLogger.Debug("cap describe instances", "len", len(found), "err", err)
			if len(found) >= control.Cap {
				instanceLogger.Warn("Simulator cap", "cap", control.Cap)
				return nil, fmt.Errorf("at capacity %v", control.Cap)
			}
		}
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	if spec.Tags == nil {
		spec.Tags = map[string]string{}
	}
	spec.Tags["simulator"] = s.plugin.String()

	key := fmt.Sprintf("%v", time.Now().UnixNano())
	description := instance.Description{
		ID:         instance.ID(key),
		Tags:       spec.Tags,
		LogicalID:  spec.LogicalID,
		Properties: types.AnyValueMust(spec.Properties),
	}
	buff, err := types.AnyValueMust(description).MarshalYAML()
	if err != nil {
		return nil, err
	}

	if control.Delay.Duration() > 0 {
		instanceLogger.Warn("Simulator delay start", "plugin", s.plugin, "duration", control.Delay.Duration())
		<-time.After(control.Delay.Duration())
		instanceLogger.Warn("Simulator delay done", "plugin", s.plugin, "duration", control.Delay.Duration())
	}

	err = s.instances.Write(description.ID, buff)
	instanceLogger.Debug("Provisioned", "id", description.ID, "spec", spec, "err", err, "plugin", s.plugin)
	return &description.ID, err
}

// Label labels the instance
func (s *instanceSimulator) Label(key instance.ID, labels map[string]string) error {
	instanceLogger.Debug("Label", "name", s.name, "instance", key, "labels", labels,
		"V", debugV, "plugin", s.plugin,
		"method", "Label")

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
	instanceLogger.Debug("Destroy", "name", s.name, "instance", instance, "context", context,
		"V", debugV, "plugin", s.plugin,
		"method", "Destroy")

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
	instanceLogger.Debug("DescribeInstances",
		"name", s.name, "labels", labels, "V", debugV2, "plugin", s.plugin,
		"method", "DescribeInstances")

	s.lock.RLock()
	defer s.lock.RUnlock()

	<-time.After(s.options.DescribeDelay)
	return s.describeInstances(labels, properties)
}

func (s *instanceSimulator) describeInstances(labels map[string]string,
	properties bool) ([]instance.Description, error) {

	matches := []instance.Description{}

	search := map[string]string{
		"simulator": s.plugin.String(),
	}
	for k, v := range labels {
		search[k] = v
	}

	err := store.Visit(s.instances,
		search,
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
