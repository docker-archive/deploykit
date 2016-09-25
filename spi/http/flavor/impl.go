package flavor

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/docker/libmachete/plugin"
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
		server.prepare,
		server.healthy,
	})
}

func (c *client) Validate(flavorProperties json.RawMessage) (flavor.AllocationMethod, error) {
	response := flavor.AllocationMethod{}
	_, err := c.c.Call(&util.HTTPEndpoint{Method: "POST", Path: "/Flavor.Validate"}, &flavorProperties, &response)
	return response, err
}

func (s *server) validate() (plugin.Endpoint, plugin.Handler) {
	return &util.HTTPEndpoint{Method: "POST", Path: "/Flavor.Validate"},

		func(vars map[string]string, body io.Reader) (result interface{}, err error) {
			buff, err := ioutil.ReadAll(body)
			if err != nil {
				return nil, err
			}

			return s.plugin.Validate(json.RawMessage(buff))
		}
}

type prepareRequest struct {
	Properties   *json.RawMessage
	InstanceSpec instance.Spec
}

func (c *client) Prepare(flavorProperties json.RawMessage, spec instance.Spec) (instance.Spec, error) {
	request := prepareRequest{Properties: &flavorProperties, InstanceSpec: spec}
	instanceSpec := instance.Spec{}
	_, err := c.c.Call(&util.HTTPEndpoint{Method: "POST", Path: "/Flavor.PreProvision"}, request, &instanceSpec)
	return instanceSpec, err
}

func (s *server) prepare() (plugin.Endpoint, plugin.Handler) {
	return &util.HTTPEndpoint{Method: "POST", Path: "/Flavor.PreProvision"},

		func(vars map[string]string, body io.Reader) (result interface{}, err error) {
			request := prepareRequest{}
			err = json.NewDecoder(body).Decode(&request)
			if err != nil {
				return nil, err
			}

			var arg1 json.RawMessage
			if request.Properties != nil {
				arg1 = *request.Properties
			}

			return s.plugin.Prepare(arg1, request.InstanceSpec)
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
