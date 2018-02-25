package ingress

import (
	"fmt"

	ingress "github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/controller/internal"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/types"
	"golang.org/x/net/context"
)

// NewController returns a controller implementation
func NewController(scope scope.Scope, leader func() stack.Leadership) controller.Controller {
	return internal.NewController(
		// the constructor
		func(spec types.Spec) (internal.Managed, error) {
			return &managed{
				leader: leader,
			}, nil
		},
		// the key function
		func(metadata types.Metadata) string {
			return metadata.Name
		},
	)
}

// NewTypedControllers return typed controllers
func NewTypedControllers(scope scope.Scope,
	leader func() stack.Leadership) func() (map[string]controller.Controller, error) {

	return (internal.NewController(
		// the constructor
		func(spec types.Spec) (internal.Managed, error) {
			return newManaged(scope, leader), nil
		},
		// the key function
		func(metadata types.Metadata) string {
			return metadata.Name
		},
	)).Controllers
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
func (m *managed) Enforce(spec types.Spec) (*types.Object, error) {
	err := m.init(spec)
	if err != nil {
		return nil, err
	}
	m.Start()
	return m.object(), nil
}

// Inspect implements internal/Managed
func (m *managed) Inspect() (*types.Object, error) {
	return m.object(), nil
}

// Free implements internal/Managed
func (m *managed) Free() (*types.Object, error) {
	if m.started() {
		m.Stop()
	}
	return m.Inspect()
}

// Terminate implements internal/Managed
func (m *managed) Terminate() (*types.Object, error) {
	if m.started() {
		m.Stop()
	}
	return m.Inspect()
}

// Start implements internal/ControlLoop
func (m *managed) Start() {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.process != nil && m.poller != nil {
		go m.poller.Run(context.Background())
	}
}

// Stop implements internal/ControlLoop
func (m *managed) Stop() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.process != nil && m.poller != nil {
		m.poller.Stop()
	}
	return nil
}

// Running implements internal/ControlLoop
func (m *managed) Running() bool {
	return m.started()
}
