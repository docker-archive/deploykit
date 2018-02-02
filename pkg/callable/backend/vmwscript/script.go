package vmwscript

import (
	"context"
	"fmt"

	"github.com/docker/infrakit/pkg/callable/backend"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/x/vmwscript"
)

var log = logutil.New("module", "playbook/vmwscript")

func init() {
	backend.Register("vmwscript", Script, nil)
}

// Script takes a list of optional parameters and returns an executable function that
// executes the payload using the VMWScript engine for automating VMWare
func Script(scope scope.Scope, test bool, opt ...interface{}) (backend.ExecFunc, error) {

	return func(ctx context.Context, script string, parameters backend.Parameters, args []string) error {

		plan := vmwscript.DeploymentPlan{}

		err := types.Decode([]byte(script), &plan)
		if err != nil {
			return err
		}

		err = plan.Validate()
		if err != nil {
			return err
		}

		if test {
			log.Info("Trial run. Printing input")
			fmt.Print(script)
			return nil
		}

		ctx2, cancel := context.WithCancel(ctx)
		defer cancel()

		client, err := vmwscript.VCenterLogin(ctx2, plan.VMWConfig)
		if err != nil {
			log.Crit("Error connecting to vCenter", "err", err)
			return err
		}

		log.Info("Starting VMwScript engine")
		plan.RunTasks(ctx2, client)
		log.Info("VMwScript has completed succesfully")

		return nil
	}, nil
}
