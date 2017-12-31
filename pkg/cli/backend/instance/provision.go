package instance

import (
	"fmt"

	"github.com/docker/infrakit/pkg/cli/backend"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

func init() {
	backend.Register("instanceProvision", Provision, nil)
}

// Provision backend requires the name of the plugin and a boolean to indicate if the content is yaml.
// It then returns an executable function based on that specification to call the named instance plugin's provision
// method.
func Provision(scope scope.Scope, test bool, opt ...interface{}) (backend.ExecFunc, error) {

	if len(opt) != 2 {
		return nil, fmt.Errorf("require params: pluginName (string) and isYAML (bool)")
	}

	name, is := opt[0].(string)
	if !is {
		return nil, fmt.Errorf("first param (pluginName) must be string")
	}

	isYAML, is := opt[1].(bool)
	if !is {
		return nil, fmt.Errorf("second param (isYAML) must be a bool")
	}

	return func(script string, cmd *cobra.Command, args []string) error {

		plugin, err := scope.Instance(name)
		if err != nil {
			return err
		}

		spec := instance.Spec{}
		if isYAML {
			y, err := types.AnyYAML([]byte(script))
			if err != nil {
				return err
			}
			if err := y.Decode(&spec); err != nil {
				return err
			}
		} else {
			if err := types.AnyString(script).Decode(&spec); err != nil {
				return err
			}
		}

		id, err := plugin.Provision(spec)
		if err != nil {
			return err
		}
		fmt.Println(*id)
		return nil
	}, nil
}
