package manager

import (
	"fmt"

	"github.com/docker/infrakit/pkg/cli/backend"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "cli/backend/manager")

func init() {
	backend.Register("managerCommit", Commit)
}

// Commit requires two parameters, first is isYAML (bool) and second is pretend (bool)
// It then returns an executable function based on that specification to call the manager's commit
// method with the content
func Commit(scope scope.Scope, test bool, opt ...interface{}) (backend.ExecFunc, error) {

	if len(opt) != 2 {
		return nil, fmt.Errorf("require params: isYAML (bool), pretend (bool)")
	}

	isYAML, is := opt[0].(bool)
	if !is {
		return nil, fmt.Errorf("first param (isYAML) must be a bool")
	}

	pretend, is := opt[1].(bool)
	if !is {
		return nil, fmt.Errorf("second param (pretend) must be a bool")
	}

	return func(script string) error {
		groups := []plugin.Spec{}
		if isYAML {
			y, err := types.AnyYAML([]byte(script))
			if err != nil {
				return err
			}
			if err := y.Decode(&groups); err != nil {
				return err
			}
		} else {
			if err := types.AnyString(script).Decode(&groups); err != nil {
				return err
			}
		}

		// Check the list of plugins
		for _, gp := range groups {

			// unmarshal the group spec
			spec := group.Spec{}
			if gp.Properties != nil {
				err := gp.Properties.Decode(&spec)
				if err != nil {
					return err
				}
			}

			target, err := scope.Group(gp.Plugin.String())
			log.Debug("commit", "plugin", gp.Plugin, "err", err, "spec", spec)

			if err != nil {
				return err
			}

			plan, err := target.CommitGroup(spec, pretend)
			if err != nil {
				return err
			}

			fmt.Println("Group", spec.ID, "with plugin", gp.Plugin, "plan:", plan)
		}
		return nil

	}, nil
}
