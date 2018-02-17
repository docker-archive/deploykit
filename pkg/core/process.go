package core

import (
	"fmt"

	"github.com/docker/infrakit/pkg/fsm"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "core")

// ProcessDefinition has the Spec and the actual behaviors like construct/ destruct
type ProcessDefinition struct {
	Spec               *types.Spec
	Constructor        Constructor
	ConstructorError   ConstructorError
	ConstructorSuccess ConstructorSuccess
	Destructor         Destructor
}

// Process manages the lifecycle of one class of objects
type Process struct {
	ProcessDefinition

	workflow  *fsm.Spec
	instances *fsm.Set

	Constructor fsm.Action

	store Objects
	scope scope.Scope

	objects map[fsm.ID]*types.Object
}

// ModelDefinition defines the fsm model
type ModelDefinition func(*Process) (*fsm.Spec, error)

// Constructor constructs the instance using the rendered input
type Constructor func(spec types.Spec, properties *types.Any) (identity *types.Identity, state *types.Any, err error)

// Destructor destroys the instance
type Destructor func(*types.Object) error

// ConstructorSuccess is the callback when construction succeeds
type ConstructorSuccess func(*types.Object, fsm.FSM) error

// ConstructorError is the callback when the destruction succeeds
type ConstructorError func(error, *types.Any, fsm.FSM) error

func check(input ProcessDefinition) ProcessDefinition {
	if input.ConstructorError == nil {
		input.ConstructorError = func(err error, properties *types.Any, instance fsm.FSM) error { return err } // noop
	}
	if input.ConstructorSuccess == nil {
		input.ConstructorSuccess = func(*types.Object, fsm.FSM) error { return nil } // noop
	}
	return input
}

// NewProcess creates a new process to manage a spec and the lifecycle of its instances.
func NewProcess(model ModelDefinition,
	input ProcessDefinition,
	store Objects,
	scope scope.Scope) (*Process, error) {

	proc := &Process{
		ProcessDefinition: check(input),
		store:             store,
		scope:             scope,
		objects:           map[fsm.ID]*types.Object{},
	}

	proc.Constructor = func(instance fsm.FSM) error {
		if proc.ProcessDefinition.Constructor == nil {
			return fmt.Errorf("no constructor %s %s", input.Spec.Kind, input.Spec.Metadata.Name)
		}

		// create an instace and resolve dependencies to compute a full spec
		obj := &types.Object{
			Spec: *proc.ProcessDefinition.Spec,
		}

		// here we resolve any dependencies in the spec
		depends, err := resolveDepends(obj, proc.store)
		if err != nil {
			return err
		}

		properties, err := renderProperties(obj, instance.ID(), depends, scope)
		if err != nil {
			return err
		}

		log.Debug("about to call constructor", "properties", properties)

		identity, state, err := proc.ProcessDefinition.Constructor(obj.Spec, properties)
		if err != nil {
			return proc.ProcessDefinition.ConstructorError(err, properties, instance)
		}

		// create an Object from the spec
		obj.Spec.Metadata.Identity = identity
		obj.State = state

		// index it
		proc.store.Add(obj)

		// associate instance ID with object
		proc.objects[instance.ID()] = obj

		// send signal
		err = proc.ProcessDefinition.ConstructorSuccess(obj, instance)
		if err != nil {
			log.Debug("err", "err", err)
			return err
		}

		return nil
	}

	workflow, err := model(proc)
	if err != nil {
		return nil, err
	}

	proc.workflow = workflow

	return proc, nil
}

// Start starts the management process of the instances
func (p *Process) Start(clock *fsm.Clock) error {
	p.instances = fsm.NewSet(p.workflow, clock, fsm.DefaultOptions(p.ProcessDefinition.Spec.Metadata.Name))
	return nil
}

// NewInstance creates an instance of the object in the initial state, calling the process constructor
func (p *Process) NewInstance(initialState fsm.Index) (fsm.FSM, error) {
	newObject := p.instances.Add(initialState)
	return newObject, p.Constructor(newObject)
}

// Instances returns a collection of fsm instances
func (p *Process) Instances() *fsm.Set {
	return p.instances
}

// Object returns the Object reference given an instance
func (p *Process) Object(instance fsm.FSM) *types.Object {
	return p.objects[instance.ID()]
}

// Instance returns an fsm instance given the Object reference
func (p *Process) Instance(object *types.Object) fsm.FSM {
	for k, v := range p.objects {
		if v == object {
			return p.instances.Get(k)
		}
	}
	return nil
}

// SpecsFromURL loads the raw specs from the URL and returns the root url and raw bytes
func SpecsFromURL(uri string) (root string, config []byte, err error) {
	buff, err := template.Fetch(uri, template.Options{})
	if err != nil {
		return uri, nil, err
	}
	buff, err = ensureJSON(buff)
	return uri, buff, err
}

func ensureJSON(buff []byte) ([]byte, error) {
	// try to decode it as json...
	var v interface{}
	err := types.AnyBytes(buff).Decode(&v)
	if err == nil {
		return buff, nil
	}

	y, err := types.AnyYAML(buff)
	if err != nil {
		return buff, err
	}
	err = y.Decode(&v)
	if err != nil {
		return nil, err
	}
	return y.Bytes(), nil
}

// NormalizeSpecs given the input bytes and its source, returns the normalized specs where
// template urls have been updated to be absolute and the specs are in dependency order.
func NormalizeSpecs(uri string, input []byte) ([]*types.Spec, error) {

	input, err := ensureJSON(input)
	if err != nil {
		return nil, err
	}

	parsed := []*types.Spec{}
	if err := types.AnyBytes(input).Decode(&parsed); err != nil {
		return nil, err
	}

	specs := []*types.Spec{}

	for _, member := range parsed {

		if err := member.Validate(); err != nil {
			return nil, err
		}

		specs = append(specs, member)
		specs = append(specs, types.Flatten(member)...)
	}

	// compute ordering
	ordered, err := types.OrderByDependency(specs)
	if err != nil {
		return nil, err
	}

	log.Debug("ordered by dependency", "count", len(ordered), "unordered", len(specs))

	// normalize all the template references with respect to the source url
	for _, spec := range ordered {

		if spec.Template != nil && !spec.Template.Absolute() {
			absolute, err := template.GetURL(uri, spec.Template.String())
			if err != nil {
				return nil, err
			}
			if u, err := types.NewURL(absolute.String()); err == nil {
				spec.Template = u
			}
		}
	}

	return ordered, nil
}
