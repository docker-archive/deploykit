package stack

import (
	"context"
	"fmt"

	"github.com/docker/infrakit/pkg/callable/backend"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/types"
)

func init() {
	backend.Register("stackEnforce", Enforce,
		func(params backend.Parameters) {
			params.String("stack", "", "Name of stack")
		})
}

// Enforce backend requires the name of the plugin and a boolean to indicate if the content is yaml.
// It then returns an executable function based on that specification to call the named instance plugin's provision
// method.
func Enforce(scope scope.Scope, test bool, opt ...interface{}) (backend.ExecFunc, error) {

	return func(ctx context.Context, script string, parameters backend.Parameters, args []string) error {

		var name string

		// Optional parameter for plugin name can be overridden by the value of the flag (--stack):
		if len(opt) > 0 {
			n, is := opt[0].(string)
			if !is {
				return fmt.Errorf("first param (stackName) must be string")
			}
			name = n
		}
		name, err := parameters.GetString("stack")
		if err != nil {
			return err
		}

		stack, err := scope.Stack(name)
		if err != nil {
			return err
		}

		specs := types.Specs{}
		if err := types.Decode([]byte(script), &specs); err != nil {
			return err
		}

		return stack.Enforce(specs)
	}, nil
}
