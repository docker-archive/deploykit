package manager

import (
	"fmt"
	"sort"

	"github.com/docker/infrakit/pkg/types"
)

// Enforce enforces infrastructure state to match that of the specs
func (m *manager) Enforce(specs []types.Spec) error {

	buff, err := types.AnyValueMust(specs).MarshalYAML()
	if err != nil {
		return err
	}

	fmt.Println(string(buff))

	return nil
}

// Specs returns the specs that are being enforced
func (m *manager) Specs() ([]types.Spec, error) {
	log.Debug("stack.Specs", "V", debugV2)

	// load the config
	config := globalSpec{}

	// load the latest version -- assumption here is that it's been persisted already.
	err := config.load(m.Options.SpecStore)
	if err != nil {
		return nil, err
	}

	specs := types.Specs{}
	for _, p := range config.data {
		specs = append(specs, p.Record.Spec)
	}

	sort.Sort(specs)

	return specs, nil
}

// Inspect returns the current state of the infrastructure
func (m *manager) Inspect() ([]types.Object, error) {
	log.Debug("stack.Inspect")
	fmt.Println(">>> INSPECT")
	return nil, nil
}

// Terminate destroys all resources associated with the specs
func (m *manager) Terminate(specs []types.Spec) error {
	log.Debug("stack.Terminate", "specs", specs)

	return fmt.Errorf("not implemented")
}
