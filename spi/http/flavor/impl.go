package flavor

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/docker/libmachete/plugin"
	"github.com/docker/libmachete/plugin/group/types"
	"github.com/docker/libmachete/plugin/util"
	"github.com/docker/libmachete/spi/flavor"
	"github.com/docker/libmachete/spi/instance"
)

type client struct {
	c plugin.Callable
}

type server struct {
	plugin flavor.Plugin
}

// PluginClient returns an instance of the Plugin
func PluginClient(c plugin.Callable) flavor.Plugin {
	return &client{c: c}
}

// PluginServer returns an instance of the Plugin
func PluginServer(p flavor.Plugin) http.Handler {

	server := &server{plugin: p}
	return util.BuildHandler([]func() (plugin.Endpoint, plugin.Handler){
		server.validate,
		server.preProvision,
		server.healthy,
	})
}

type validateRequest struct {
	Properties *json.RawMessage
	GroupSpec  types.Schema
}

type validateResponse struct {
	InstanceIDKind flavor.InstanceIDKind
}

func (c *client) Validate(flavorProperties json.RawMessage, parsed types.Schema) (flavor.InstanceIDKind, error) {
	request := validateRequest{Properties: &flavorProperties, GroupSpec: parsed}
	response := validateResponse{}
	_, err := c.c.Call(&util.HTTPEndpoint{Method: "POST", Path: "/Flavor.Validate"}, request, &response)
	return response.InstanceIDKind, err
}

func (s *server) validate() (plugin.Endpoint, plugin.Handler) {
	return &util.HTTPEndpoint{Method: "POST", Path: "/Flavor.Validate"},

		func(vars map[string]string, body io.Reader) (result interface{}, err error) {
			request := validateRequest{}
			err = json.NewDecoder(body).Decode(&request)
			if err != nil {
				return nil, err
			}

			var arg1 json.RawMessage
			if request.Properties != nil {
				arg1 = *request.Properties
			}

			kind, err := s.plugin.Validate(arg1, request.GroupSpec)
			return validateResponse{InstanceIDKind: kind}, err
		}
}

type preProvisionRequest struct {
	Properties   *json.RawMessage
	InstanceSpec instance.Spec
}

func (c *client) PreProvision(flavorProperties json.RawMessage, spec instance.Spec) (instance.Spec, error) {
	request := preProvisionRequest{Properties: &flavorProperties, InstanceSpec: spec}
	instanceSpec := instance.Spec{}
	_, err := c.c.Call(&util.HTTPEndpoint{Method: "POST", Path: "/Flavor.PreProvision"}, request, &instanceSpec)
	return instanceSpec, err
}

func (s *server) preProvision() (plugin.Endpoint, plugin.Handler) {
	return &util.HTTPEndpoint{Method: "POST", Path: "/Flavor.PreProvision"},

		func(vars map[string]string, body io.Reader) (result interface{}, err error) {
			request := preProvisionRequest{}
			err = json.NewDecoder(body).Decode(&request)
			if err != nil {
				return nil, err
			}

			var arg1 json.RawMessage
			if request.Properties != nil {
				arg1 = *request.Properties
			}

			return s.plugin.PreProvision(arg1, request.InstanceSpec)
		}
}

type healthResponse struct {
	Healthy  bool
	Instance instance.Description
}

func (c *client) Healthy(inst instance.Description) (bool, error) {
	response := healthResponse{}
	_, err := c.c.Call(&util.HTTPEndpoint{Method: "POST", Path: "/Flavor.Healthy"}, inst, &response)
	return response.Healthy, err
}

func (s *server) healthy() (plugin.Endpoint, plugin.Handler) {
	return &util.HTTPEndpoint{Method: "POST", Path: "/Flavor.Healthy"},

		func(vars map[string]string, body io.Reader) (result interface{}, err error) {
			inst := instance.Description{}
			err = json.NewDecoder(body).Decode(&inst)
			if err != nil {
				return nil, err
			}
			healthy, err := s.plugin.Healthy(inst)
			return healthResponse{Healthy: healthy, Instance: inst}, err
		}
}
