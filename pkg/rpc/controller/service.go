package controller

import (
	"fmt"
	"net/http"

	"github.com/docker/infrakit/pkg/controller"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi"
)

var log = logutil.New("module", "rpc/controller")

// ServerWithTypes returns a Controller map
func ServerWithTypes(subcontrollers func() (map[string]controller.Controller, error)) *Controller {
	return &Controller{subcontrollers: subcontrollers}
}

// Server returns a Controller that conforms to the net/rpc rpc call convention.
func Server(c controller.Controller) *Controller {
	return ServerWithTypes(func() (map[string]controller.Controller, error) {
		return map[string]controller.Controller{
			".": c,
		}, nil
	})
}

// Controller is the exported type for json-rpc
type Controller struct {
	subcontrollers func() (map[string]controller.Controller, error)
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (c *Controller) ImplementedInterface() spi.InterfaceSpec {
	return controller.InterfaceSpec
}

// Types returns the types exposed by this kind of RPC service
func (c *Controller) Types() []string {
	m, err := c.subcontrollers()
	log.Warn(">>>>>>>>> TYPES", "m", m, "err", err)
	if err != nil {
		return nil
	}
	types := []string{}
	for k := range m {
		types = append(types, k)
	}
	return types
}

func (c *Controller) subController(name plugin.Name) (controller.Controller, error) {
	m, err := c.subcontrollers()
	if err != nil {
		return nil, err
	}
	_, subtype := name.GetLookupAndType()
	key := subtype
	if subtype == "." || subtype == "" {
		key = "."
	}
	if p, has := m[key]; has {
		return p, nil
	}
	return nil, fmt.Errorf("no-controller:%v", name)
}

// Plan is the rpc method for Plan
func (c *Controller) Plan(_ *http.Request, req *ChangeRequest, resp *ChangeResponse) error {
	resp.Name = req.Name
	subcontroller, err := c.subController(req.Name)
	if err != nil {
		return err
	}
	object, plan, err := subcontroller.Plan(req.Operation, req.Spec)
	if err == nil {
		resp.Object = object
		resp.Plan = plan
	}
	return err
}

// Commit is the rpc method for Commit
func (c *Controller) Commit(_ *http.Request, req *ChangeRequest, resp *ChangeResponse) error {
	resp.Name = req.Name
	subcontroller, err := c.subController(req.Name)
	if err != nil {
		return err
	}
	object, err := subcontroller.Commit(req.Operation, req.Spec)
	if err == nil {
		resp.Object = object
	}
	return err
}

// Describe is the rpc method for Describe
func (c *Controller) Describe(_ *http.Request, req *FindRequest, resp *FindResponse) error {
	resp.Name = req.Name
	subcontroller, err := c.subController(req.Name)
	if err != nil {
		return err
	}
	objects, err := subcontroller.Describe(req.Metadata)
	if err == nil {
		resp.Objects = objects
	}
	return err
}

// Free is the rpc method for Free
func (c *Controller) Free(_ *http.Request, req *FindRequest, resp *FindResponse) error {
	resp.Name = req.Name
	subcontroller, err := c.subController(req.Name)
	if err != nil {
		return err
	}
	objects, err := subcontroller.Free(req.Metadata)
	if err == nil {
		resp.Objects = objects
	}
	return err
}
