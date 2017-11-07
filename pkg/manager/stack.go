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

// Inspect returns the current state of the infrastructure
func (m *manager) Inspect() ([]types.Object, error) {
	return nil, nil
}

// Terminate destroys all resources associated with the specs
func (m *manager) Terminate(specs []types.Spec) error {
	return fmt.Errorf("not implemented")
}
