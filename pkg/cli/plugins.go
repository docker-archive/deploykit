package cli

import (
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch"
	"github.com/docker/infrakit/pkg/run/manager"
	"github.com/docker/infrakit/pkg/types"
)

// PluginManager returns the plugin manager for running plugins locally.
func PluginManager(plugins func() discovery.Plugins,
	services *Services, configURL string) (*manager.Manager, error) {

	parsedRules := []launch.Rule{}

	if configURL != "" {
		buff, err := services.ProcessTemplate(configURL)
		if err != nil {
			return nil, err
		}
		view, err := services.ToJSON([]byte(buff))
		if err != nil {
			return nil, err
		}
		configs := types.AnyBytes(view)
		err = configs.Decode(&parsedRules)
		if err != nil {
			return nil, err
		}
	}
	return manager.ManagePlugins(parsedRules, plugins, true, 5*time.Second)
}
