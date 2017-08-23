package manager

import (
	"fmt"

	"github.com/docker/infrakit/pkg/controller"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/types"
)

// GroupControllers returns a map of *scoped* group controllers by ID of the group.
func (m *manager) GroupControllers() (map[string]controller.Controller, error) {
	base := newControllerProxy(nil, m.Plugin)
	controllers := map[string]controller.Controller{
		"": base,
	}
	all, err := m.Plugin.InspectGroups()
	if err != nil {
		return controllers, nil
	}
	for _, spec := range all {
		gid := spec.ID
		controllers[string(gid)] = newControllerProxy(&gid, m.Plugin)
	}
	log.Debug("GroupControllers", "map", controllers, "V", logutil.V(500))
	return controllers, nil
}

// newControllerProxy returns a Controller implementation that optionally
// enforces scoping by group ID if provided
func newControllerProxy(id *group.ID, g group.Plugin) controller.Controller {
	return &pController{
		plugin: g,
		scope:  id,
	}
}

// This controller is used to implement a generic controller *as well as* a named controller
// for a group.  When id is specified, the controller is scoped to the id.  When input is missing
// id, it will be injected.  If input has mismatched id, requests will error.
type pController struct {
	scope  *group.ID
	plugin group.Plugin
}

func (c *pController) translateSpec(spec types.Spec) (group.Spec, error) {
	gSpec := group.Spec{
		Properties: spec.Properties,
	}
	if c.scope == nil {
		if spec.Metadata.Name == "" {
			return gSpec, fmt.Errorf("no group name")
		}
		gSpec.ID = group.ID(spec.Metadata.Name)
		return gSpec, nil
	}

	if spec.Metadata.Name != string(*c.scope) {
		return group.Spec{}, fmt.Errorf("wrong group: %v", *c.scope)
	}

	gSpec.ID = *c.scope
	return gSpec, nil
}

func objectFromSpec(spec types.Spec) types.Object {
	return types.Object{
		Spec: spec,
	}
}

func (c *pController) Plan(operation controller.Operation,
	spec types.Spec) (object types.Object, plan controller.Plan, err error) {

	gSpec, e := c.translateSpec(spec)
	if e != nil {
		err = e
		return
	}

	plan = controller.Plan{}
	object = objectFromSpec(spec)
	if resp, cerr := c.plugin.CommitGroup(gSpec, true); cerr == nil {
		plan.Message = []string{resp}
	} else {
		err = cerr
	}
	return
}

func (c *pController) Commit(operation controller.Operation, spec types.Spec) (object types.Object, err error) {

	gSpec, e := c.translateSpec(spec)
	if e != nil {
		err = e
		return
	}

	object = objectFromSpec(spec)
	switch operation {
	case controller.Manage:
		_, err = c.plugin.CommitGroup(gSpec, false)
	case controller.Destroy:
		err = c.plugin.DestroyGroup(group.ID(spec.Metadata.Name))
	}
	return
}

func (c *pController) helpFind(search *types.Metadata) ([]string, map[string]group.Spec, error) {
	specs := map[string]group.Spec{}

	all, err := c.plugin.InspectGroups()
	if err != nil {
		return nil, nil, err
	}
	for _, gs := range all {
		specs[string(gs.ID)] = gs
	}

	ids := []string{}
	if search == nil {
		for k := range specs {
			ids = append(ids, k)
		}
	} else {
		ids = append(ids, search.Name)
	}

	// Scoping by group id
	if c.scope != nil {
		ids = []string{string(*c.scope)}

	}
	return ids, specs, nil
}

func (c *pController) Describe(search *types.Metadata) (objects []types.Object, err error) {
	var ids []string
	var specs map[string]group.Spec

	ids, specs, err = c.helpFind(search)
	if err != nil {
		return
	}

	objects = []types.Object{}

	for _, id := range ids {

		var desc group.Description
		desc, err = c.plugin.DescribeGroup(group.ID(id))
		if err != nil {
			return
		}
		state := types.Object{
			Spec: types.Spec{
				Kind:    "group",
				Version: group.InterfaceSpec.Encode(),
				Metadata: types.Metadata{
					Identity: &types.Identity{ID: id},
					Name:     id,
				},
				Properties: types.AnyValueMust(specs[id]),
			},
			State: types.AnyValueMust(desc),
		}
		objects = append(objects, state)
	}
	return
}

func (c *pController) Free(search *types.Metadata) (objects []types.Object, err error) {
	objects, err = c.Describe(search)
	err = c.plugin.FreeGroup(group.ID(search.Name))
	return
}
