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

	return nil
}

// Specs returns the specs that are being enforced
func (m *manager) Specs() ([]types.Spec, error) {
	log.Debug("stack.Specs")
	fmt.Println(">>> SPECS")
	return nil, nil
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
