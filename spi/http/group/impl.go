package group

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/docker/libmachete/plugin"
	"github.com/docker/libmachete/plugin/util"
	"github.com/docker/libmachete/spi/group"
)

type client struct {
	c plugin.Callable
}

type server struct {
	plugin group.Plugin
}

// PluginClient returns an instance of the Plugin
func PluginClient(c plugin.Callable) group.Plugin {
	return &client{c: c}
}

// PluginServer returns an instance of the Plugin
func PluginServer(p group.Plugin) http.Handler {

	server := &server{plugin: p}
	return util.BuildHandler([]func() (plugin.Endpoint, plugin.Handler){
		server.watchGroup,
		server.unwatchGroup,
		server.inspectGroup,
		server.describeUpdate,
		server.updateGroup,
		server.stopUpdate,
		server.destroyGroup,
	})
}

func (c *client) WatchGroup(grp group.Configuration) error {
	_, err := c.c.Call(&util.HTTPEndpoint{Method: "POST", Path: "/Watch"}, grp, nil)
	return err
}

func (s *server) watchGroup() (plugin.Endpoint, plugin.Handler) {
	return &util.HTTPEndpoint{Method: "POST", Path: "/Watch"},

		func(vars map[string]string, body io.Reader) (result interface{}, err error) {
			config := group.Configuration{}
			err = json.NewDecoder(body).Decode(&config)
			if err != nil {
				return nil, err
			}
			err = s.plugin.WatchGroup(config)
			return nil, err
		}
}

func (c *client) UnwatchGroup(id group.ID) error {
	_, err := c.c.Call(&util.HTTPEndpoint{Method: "POST", Path: fmt.Sprintf("/Unwatch/%v", id)}, nil, nil)
	return err
}

func (s *server) unwatchGroup() (plugin.Endpoint, plugin.Handler) {
	return &util.HTTPEndpoint{Method: "POST", Path: "/Unwatch/{id}"},

		func(vars map[string]string, body io.Reader) (result interface{}, err error) {
			err = s.plugin.UnwatchGroup(group.ID(vars["id"]))
			return nil, err
		}
}

func (c *client) InspectGroup(id group.ID) (group.Description, error) {
	description := group.Description{}
	_, err := c.c.Call(&util.HTTPEndpoint{Method: "POST", Path: fmt.Sprintf("/Inspect/%v", id)}, nil, &description)
	return description, err
}

func (s *server) inspectGroup() (plugin.Endpoint, plugin.Handler) {
	return &util.HTTPEndpoint{Method: "POST", Path: "/Inspect/{id}"},

		func(vars map[string]string, body io.Reader) (result interface{}, err error) {
			return s.plugin.InspectGroup(group.ID(vars["id"]))
		}
}

func (c *client) DescribeUpdate(updated group.Configuration) (string, error) {
	envelope := map[string]string{}
	_, err := c.c.Call(&util.HTTPEndpoint{Method: "POST", Path: "/DescribeUpdate"}, updated, &envelope)
	return envelope["message"], err
}

func (s *server) describeUpdate() (plugin.Endpoint, plugin.Handler) {
	return &util.HTTPEndpoint{Method: "POST", Path: "/DescribeUpdate"},

		func(vars map[string]string, body io.Reader) (result interface{}, err error) {
			updated := group.Configuration{}
			err = json.NewDecoder(body).Decode(&updated)
			if err != nil {
				return nil, err
			}
			message, err := s.plugin.DescribeUpdate(updated)
			if err != nil {
				return nil, err
			}
			// Use a wrapper
			return map[string]string{
				"message": message,
			}, nil
		}
}

func (c *client) UpdateGroup(updated group.Configuration) error {
	_, err := c.c.Call(&util.HTTPEndpoint{Method: "POST", Path: "/UpdateGroup"}, updated, nil)
	return err
}

func (s *server) updateGroup() (plugin.Endpoint, plugin.Handler) {
	return &util.HTTPEndpoint{Method: "POST", Path: "/UpdateGroup"},

		func(vars map[string]string, body io.Reader) (result interface{}, err error) {
			updated := group.Configuration{}
			err = json.NewDecoder(body).Decode(&updated)
			if err != nil {
				return nil, err
			}
			err = s.plugin.UpdateGroup(updated)
			return nil, err
		}
}

func (c *client) StopUpdate(id group.ID) error {
	_, err := c.c.Call(&util.HTTPEndpoint{Method: "POST", Path: fmt.Sprintf("/StopUpdate/%v", id)}, nil, nil)
	return err
}

func (s *server) stopUpdate() (plugin.Endpoint, plugin.Handler) {
	return &util.HTTPEndpoint{Method: "POST", Path: "/StopUpdate"},

		func(vars map[string]string, body io.Reader) (result interface{}, err error) {
			err = s.plugin.StopUpdate(group.ID(vars["id"]))
			return nil, err
		}
}

func (c *client) DestroyGroup(id group.ID) error {
	_, err := c.c.Call(&util.HTTPEndpoint{Method: "POST", Path: fmt.Sprintf("/DestroyGroup/%v", id)}, nil, nil)
	return err
}

func (s *server) destroyGroup() (plugin.Endpoint, plugin.Handler) {
	return &util.HTTPEndpoint{Method: "POST", Path: "/DestroyGroup/{id}"},

		func(vars map[string]string, body io.Reader) (result interface{}, err error) {
			err = s.plugin.DestroyGroup(group.ID(vars["id"]))
			return nil, err
		}
}
