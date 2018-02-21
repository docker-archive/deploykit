package controller

import (
	"net/http"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/rpc/internal"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "rpc/controller")

// ServerWithNames returns a Controller map
func ServerWithNames(subcontrollers func() (map[string]controller.Controller, error)) *Controller {

	keyed := internal.ServeKeyed(

		// This is where templates would be nice...
		func() (map[string]interface{}, error) {
			m, err := subcontrollers()
			if err != nil {
				return nil, err
			}
			out := map[string]interface{}{}
			for k, v := range m {
				out[string(k)] = v
			}
			return out, nil
		},
	)

	return &Controller{keyed: keyed}
}

// Server returns a Controller that conforms to the net/rpc rpc call convention.
func Server(c controller.Controller) *Controller {
	return &Controller{keyed: internal.ServeSingle(c)}
}

// Controller is the exported type for json-rpc
type Controller struct {
	keyed *internal.Keyed
}

// VendorInfo returns a metadata object about the plugin, if the plugin implements it.  See plugin.Vendor
func (c *Controller) VendorInfo() *spi.VendorInfo {
	base, _ := c.keyed.Keyed(plugin.Name("."))
	if m, is := base.(spi.Vendor); is {
		return m.VendorInfo()
	}
	return nil
}

// ExampleProperties returns an example properties used by the plugin
func (c *Controller) ExampleProperties() *types.Any {
	base, _ := c.keyed.Keyed(plugin.Name("."))
	if i, is := base.(spi.InputExample); is {
		return i.ExampleProperties()
	}
	return nil
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (c *Controller) ImplementedInterface() spi.InterfaceSpec {
	return controller.InterfaceSpec
}

// Objects returns the objects exposed by this kind of RPC service
func (c *Controller) Objects() []rpc.Object {
	return c.keyed.Objects()
}

// Plan is the rpc method for Plan
func (c *Controller) Plan(_ *http.Request, req *ChangeRequest, resp *ChangeResponse) error {

	return c.keyed.Do(req, func(v interface{}) error {
		resp.Name = req.Name
		object, plan, err := v.(controller.Controller).Plan(req.Operation, req.Spec)
		if err == nil {
			resp.Object = object
			resp.Plan = plan
		}
		return err
	})

}

// Commit is the rpc method for Commit
func (c *Controller) Commit(_ *http.Request, req *ChangeRequest, resp *ChangeResponse) error {

	return c.keyed.Do(req, func(v interface{}) error {
		resp.Name = req.Name
		object, err := v.(controller.Controller).Commit(req.Operation, req.Spec)
		if err == nil {
			resp.Object = object
		}
		return err
	})
}

// Describe is the rpc method for Describe
func (c *Controller) Describe(_ *http.Request, req *FindRequest, resp *FindResponse) error {
	pn, _ := req.Plugin()
	return c.keyed.Do(req, func(v interface{}) error {
		log.Debug("Describe", "req", req, "p", pn, "v", v, "meta", req.Metadata)

		resp.Name = req.Name
		objects, err := v.(controller.Controller).Describe(req.Metadata)
		if err == nil {
			resp.Objects = objects
		}
		return err
	})
}

// Free is the rpc method for Free
func (c *Controller) Free(_ *http.Request, req *FindRequest, resp *FindResponse) error {

	return c.keyed.Do(req, func(v interface{}) error {
		resp.Name = req.Name
		objects, err := v.(controller.Controller).Free(req.Metadata)
		if err == nil {
			resp.Objects = objects
		}
		return err
	})
}
