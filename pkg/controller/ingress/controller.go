package ingress

import (
	"fmt"

	"github.com/docker/infrakit/pkg/controller"
	ingress "github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/controller/internal"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/types"
)

// NewController returns a controller implementation
func NewController(plugins func() discovery.Plugins, leader manager.Leadership) controller.Controller {
	return internal.NewController(
		leader,
		// the constructor
		func(spec types.Spec) (internal.Managed, error) {
			return &managed{
				Leadership: leader,
			}, nil
		},
		// the key function
		func(metadata types.Metadata) string {
			return metadata.Name
		},
	)
}

// NewTypedControllers return typed controllers
func NewTypedControllers(plugins func() discovery.Plugins,
	leader manager.Leadership) func() (map[string]controller.Controller, error) {

	return (internal.NewController(
		leader,
		// the constructor
		func(spec types.Spec) (internal.Managed, error) {
			return newManaged(plugins, leader), nil
		},
		// the key function
		func(metadata types.Metadata) string {
			return metadata.Name
		},
	)).ManagedObjects
}

// Plan implements internal/Managed
func (m *managed) Plan(operation controller.Operation, spec types.Spec) (*types.Object, *controller.Plan, error) {

	// Do basic validation
	// TODO(chungers) - provide detail plan for reconciling between current object state (types.Object) and input spec

	if operation == controller.Destroy {
		return m.object(), nil, nil
	}

	if spec.Properties == nil {
		return nil, nil, fmt.Errorf("missing properties")
	}
	ispec := ingress.Spec{}
	err := spec.Properties.Decode(&ispec)
	if err != nil {
		return nil, nil, err
	}

	// TODO - get current state to get all the routes and backends
	return &types.Object{
		Spec: spec,
	}, &controller.Plan{}, nil
}

// Manage implements internal/Managed
func (m *managed) Manage(spec types.Spec) (*types.Object, error) {
	err := m.init(spec)
	if err != nil {
		return nil, err
	}
	m.start()
	return m.object(), nil
}

// Object implements internal/Managed
func (m *managed) Object() (*types.Object, error) {
	fmt.Println(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>", m)
	return m.object(), nil
}

// Free implements internal/Managed
func (m *managed) Free() (*types.Object, error) {
	if m.started() {
		m.stop()
	}
	return m.Object()
}

// Dispose implements internal/Managed
func (m *managed) Dispose() (*types.Object, error) {
	if m.started() {
		m.stop()
	}
	return m.Object()
}
