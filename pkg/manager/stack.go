package manager

import (
	"fmt"

	"github.com/docker/infrakit/pkg/types"
)

// Enforce enforces infrastructure state to match that of the specs
func (m *manager) Enforce(specs []types.Spec) error {

	buff, err := types.AnyValueMust(specs).MarshalYAML()
	if err != nil {
		return err
	}

	fmt.Println(string(buff))

	return fmt.Errorf("not implemented -- commit to individual plugins")
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

	return config.toSpecs(), nil
}

// Inspect returns the current state of the infrastructure
func (m *manager) Inspect() ([]types.Object, error) {
	log.Debug("stack.Inspect")
	return nil, fmt.Errorf("not implemented -- coming soon")
}

// Terminate destroys all resources associated with the specs
func (m *manager) Terminate(specs []types.Spec) error {
	log.Debug("stack.Terminate", "specs", specs)
	return fmt.Errorf("not implemented -- destroy via direct calls to plugins")
}
