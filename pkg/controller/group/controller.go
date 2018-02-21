package group

import (
	"fmt"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/types"
)

// AsController returns a Controller, possibly with a scope of the group ID.
func AsController(addr plugin.Addressable, g group.Plugin) controller.Controller {
	var scope *group.ID
	if addr.Instance() != "" {
		gid := group.ID(addr.Instance())
		scope = &gid
	}
	return &adapter{
		Addressable: addr, // address of the plugin backend
		scope:       scope,
		plugin:      g,
	}
}

// This controller is used to implement a generic controller *as well as* a named controller
// for a group.  When id is specified, the controller is scoped to the id.  When input is missing
// id, it will be injected.  If input has mismatched id, requests will error.
type adapter struct {
	plugin.Addressable
	scope  *group.ID
	plugin group.Plugin
}

func (c *adapter) translateSpec(spec types.Spec) (group.Spec, error) {
	gSpec := group.Spec{
		Properties: spec.Properties,
	}

	addressable := plugin.AsAddressable(spec)
	if c.scope == nil {
		if addressable.Instance() == "" {
			return gSpec, fmt.Errorf("no group name")
		}
		gSpec.ID = group.ID(addressable.Instance())
		return gSpec, nil
	}
	if addressable.Instance() != string(*c.scope) {
		return group.Spec{}, fmt.Errorf("wrong group: instance=%v scope=%v", addressable.Instance(), *c.scope)
	}
	gSpec.ID = *c.scope

	return gSpec, nil
}

func buildObject(spec types.Spec, desc group.Description) (types.Object, error) {
	state, err := types.AnyValue(desc)
	if err != nil {
		return types.Object{}, err
	}
	return types.Object{
		Spec:  spec,
		State: state,
	}, nil
}

func (c *adapter) fromGroupSpec(gspec group.Spec) types.Spec {
	return types.Spec{
		Kind:    c.Kind(),
		Version: group.InterfaceSpec.Encode(),
		Metadata: types.Metadata{
			Identity: &types.Identity{ID: string(gspec.ID)},
			Name:     c.Plugin().WithType(gspec.ID).String(),
		},
		Properties: gspec.Properties,
		Options:    nil, // TODO(chungers) -- here's a loss of information in the old format
	}
}

func (c *adapter) helpFind(search *types.Metadata) (gspecs map[group.ID]group.Spec, err error) {
	gspecs = map[group.ID]group.Spec{}

	all := []group.Spec{}
	all, err = c.plugin.InspectGroups()
	if err != nil {
		return
	}

	for _, gspec := range all {
		gspecs[gspec.ID] = gspec
		if c.scope != nil && *c.scope == gspec.ID {
			break
		}
	}
	return
}

func (c *adapter) Plan(operation controller.Operation,
	spec types.Spec) (object types.Object, plan controller.Plan, err error) {

	gSpec, e := c.translateSpec(spec)
	if e != nil {
		err = e
		return
	}

	plan = controller.Plan{}
	objects, e := c.Describe(&spec.Metadata)
	if e != nil {
		err = e
		return
	}

	if len(objects) == 0 {
		object, err = buildObject(spec, group.Description{})
		if err != nil {
			return
		}
		plan.Message = []string{"create-new"}
	} else if len(objects) == 1 {
		object = objects[0]
		plan.Message = []string{"update-existing"}
	} else {
		err = fmt.Errorf("change affects more than one object")
		return
	}

	if resp, cerr := c.plugin.CommitGroup(gSpec, true); cerr == nil {
		plan.Message = []string{resp}
	} else {
		err = cerr
	}
	return
}

func (c *adapter) Commit(operation controller.Operation, spec types.Spec) (object types.Object, err error) {
	gSpec, e := c.translateSpec(spec)
	if e != nil {
		err = e
		return
	}

	objects, e := c.Describe(&spec.Metadata)
	if e != nil {
		err = e
		return
	}

	if len(objects) == 0 {
		object, err = buildObject(spec, group.Description{})
		if err != nil {
			return
		}
	} else if len(objects) == 1 {
		object = objects[0]
	} else {
		err = fmt.Errorf("change affects more than one object")
		return
	}

	switch operation {
	case controller.Enforce:
		_, err = c.plugin.CommitGroup(gSpec, false)
	case controller.Destroy:
		err = c.plugin.DestroyGroup(group.ID(spec.Metadata.Name))
	}
	return
}

func (c *adapter) Describe(search *types.Metadata) (objects []types.Object, err error) {
	var gspecs map[group.ID]group.Spec
	gspecs, err = c.helpFind(search)
	if err != nil {
		return
	}

	match := func(gid group.ID) bool {
		if search == nil {
			return true
		}
		query := plugin.NewAddressableFromMetadata(c.Kind(), *search)
		return query.Instance() == string(gid)
	}

	objects = []types.Object{}
	for gid, gspec := range gspecs {
		if match(gid) {
			var desc group.Description
			var object types.Object

			desc, err = c.plugin.DescribeGroup(gid)
			if err != nil {
				return
			}
			object, e := buildObject(c.fromGroupSpec(gspec), desc)
			if e != nil {
				err = e
				return
			}
			objects = append(objects, object)
		}
	}
	return
}

func (c *adapter) Specs(search *types.Metadata) (specs []types.Spec, err error) {
	var gspecs map[group.ID]group.Spec
	gspecs, err = c.helpFind(search)
	if err != nil {
		return
	}

	match := func(gid group.ID) bool {
		if search == nil {
			return true
		}
		query := plugin.NewAddressableFromMetadata(c.Kind(), *search)
		return query.Instance() == string(gid)
	}

	specs = []types.Spec{}
	for gid, gspec := range gspecs {
		if match(gid) {
			specs = append(specs, c.fromGroupSpec(gspec))
		}
	}
	return
}

func (c *adapter) Free(search *types.Metadata) (objects []types.Object, err error) {
	objects, err = c.Describe(search)
	if err != nil {
		return
	}
	for _, object := range objects {
		addr := plugin.AsAddressable(object.Spec)
		err = c.plugin.FreeGroup(group.ID(addr.Instance()))
		if err != nil {
			return
		}
	}
	return
}

func (c *adapter) Terminate(search *types.Metadata) (objects []types.Object, err error) {
	objects, err = c.Describe(search)
	if err != nil {
		return
	}
	for _, object := range objects {
		addr := plugin.AsAddressable(object.Spec)
		err = c.plugin.DestroyGroup(group.ID(addr.Instance()))
		if err != nil {
			return
		}
	}
	return
}
